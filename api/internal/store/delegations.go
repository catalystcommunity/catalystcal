package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Delegation lets a delegate act as the principal within scopes.
type Delegation struct {
	ID              string
	PrincipalID     string
	DelegateDomain  string
	DelegateSubject string
	Scopes          []string
	// CalendarIDs limits the delegation. Empty means every calendar of the
	// principal.
	CalendarIDs []string
	Note        string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RevokedAt   *time.Time
}

// LiveAt is true when the delegation is not revoked and not expired.
func (d Delegation) LiveAt(now time.Time) bool {
	return d.RevokedAt == nil && (d.ExpiresAt == nil || d.ExpiresAt.After(now))
}

const delegationCols = `id, principal_id, delegate_domain, delegate_subject, scopes, note, expires_at, created_at, updated_at, revoked_at`

func scanDelegation(row interface{ Scan(...any) error }) (Delegation, error) {
	var d Delegation
	var scopes string
	var note, expires, revoked sql.NullString
	var created, updated string
	if err := row.Scan(&d.ID, &d.PrincipalID, &d.DelegateDomain, &d.DelegateSubject, &scopes, &note,
		&expires, &created, &updated, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Delegation{}, ErrNotFound
		}
		return Delegation{}, err
	}
	d.Scopes = strings.Fields(scopes)
	d.Note = strOf(note)
	d.ExpiresAt, d.RevokedAt = timeOf(expires), timeOf(revoked)
	d.CreatedAt, d.UpdatedAt = mustTime(created), mustTime(updated)
	return d, nil
}

func (s *Store) delegationList(ctx context.Context, query string, args ...any) ([]Delegation, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var out []Delegation
	for rows.Next() {
		d, err := scanDelegation(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		ids, err := s.delegationCalendars(ctx, s.DB, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].CalendarIDs = ids
	}
	return out, nil
}

func (s *Store) delegationCalendars(ctx context.Context, q queryer, id string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT calendar_id FROM delegation_calendars WHERE delegation_id = ? ORDER BY calendar_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DelegationCalendars returns the calendar limit of a delegation (empty =
// every calendar of the principal).
func (s *Store) DelegationCalendars(ctx context.Context, id string) ([]string, error) {
	return s.delegationCalendars(ctx, s.DB, id)
}

// GetDelegation reads one delegation with its calendar list.
func (s *Store) GetDelegation(ctx context.Context, id string) (Delegation, error) {
	d, err := scanDelegation(s.DB.QueryRowContext(ctx, `SELECT `+delegationCols+` FROM delegations WHERE id = ?`, id))
	if err != nil {
		return Delegation{}, err
	}
	d.CalendarIDs, err = s.delegationCalendars(ctx, s.DB, id)
	return d, err
}

// CreateDelegation stores a delegation and its calendar list. An expired
// but unrevoked delegation for the same pair is revoked first, so the
// one-live-pair index does not refuse the new one. A live one gives
// ErrConflict.
func (s *Store) CreateDelegation(ctx context.Context, d Delegation) (Delegation, error) {
	d.DelegateDomain = strings.ToLower(d.DelegateDomain)
	now := s.now()
	d.ID = NewID()
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE delegations SET revoked_at = ?
			WHERE principal_id = ? AND delegate_domain = ? AND delegate_subject = ?
			AND revoked_at IS NULL AND expires_at IS NOT NULL AND expires_at <= ?`,
			FormatTime(now), d.PrincipalID, d.DelegateDomain, d.DelegateSubject, FormatTime(now)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO delegations
			(id, principal_id, delegate_domain, delegate_subject, scopes, note, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.ID, d.PrincipalID, d.DelegateDomain, d.DelegateSubject, strings.Join(d.Scopes, " "),
			nullStr(d.Note), nullTime(d.ExpiresAt), FormatTime(now), FormatTime(now))
		if isUnique(err) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		return setDelegationCalendars(ctx, tx, d.ID, d.CalendarIDs)
	})
	if err != nil {
		return Delegation{}, err
	}
	return s.GetDelegation(ctx, d.ID)
}

func setDelegationCalendars(ctx context.Context, tx *sql.Tx, id string, cals []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM delegation_calendars WHERE delegation_id = ?`, id); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, c := range cals {
		if seen[c] {
			continue
		}
		seen[c] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO delegation_calendars (delegation_id, calendar_id) VALUES (?, ?)`, id, c); err != nil {
			return err
		}
	}
	return nil
}

// UpdateDelegation replaces scopes, calendars, note, and expiry.
func (s *Store) UpdateDelegation(ctx context.Context, d Delegation) (Delegation, error) {
	now := s.now()
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE delegations SET scopes = ?, note = ?, expires_at = ?, updated_at = ?
			WHERE id = ? AND revoked_at IS NULL`,
			strings.Join(d.Scopes, " "), nullStr(d.Note), nullTime(d.ExpiresAt), FormatTime(now), d.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return setDelegationCalendars(ctx, tx, d.ID, d.CalendarIDs)
	})
	if err != nil {
		return Delegation{}, err
	}
	return s.GetDelegation(ctx, d.ID)
}

// RevokeDelegation sets revoked_at.
func (s *Store) RevokeDelegation(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE delegations SET revoked_at = ?, updated_at = ? WHERE id = ? AND revoked_at IS NULL`,
		FormatTime(s.now()), FormatTime(s.now()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDelegationsByPrincipal returns delegations the principal gave.
func (s *Store) ListDelegationsByPrincipal(ctx context.Context, principalID string) ([]Delegation, error) {
	return s.delegationList(ctx, `SELECT `+delegationCols+` FROM delegations WHERE principal_id = ?
		ORDER BY created_at DESC, id`, principalID)
}

// ListDelegationsByDelegate returns delegations given to an identity.
func (s *Store) ListDelegationsByDelegate(ctx context.Context, domain, subject string) ([]Delegation, error) {
	return s.delegationList(ctx, `SELECT `+delegationCols+` FROM delegations WHERE delegate_domain = ? AND delegate_subject = ?
		ORDER BY created_at DESC, id`, strings.ToLower(domain), subject)
}

// FindLiveDelegation returns the live delegation from principal to the
// delegate identity, or ErrNotFound.
func (s *Store) FindLiveDelegation(ctx context.Context, principalID, domain, subject string) (Delegation, error) {
	now := FormatTime(s.now())
	d, err := scanDelegation(s.DB.QueryRowContext(ctx, `SELECT `+delegationCols+` FROM delegations
		WHERE principal_id = ? AND delegate_domain = ? AND delegate_subject = ?
		AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`,
		principalID, strings.ToLower(domain), subject, now))
	if err != nil {
		return Delegation{}, err
	}
	d.CalendarIDs, err = s.delegationCalendars(ctx, s.DB, d.ID)
	return d, err
}
