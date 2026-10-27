package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Share gives a grantee a role on one calendar.
type Share struct {
	ID             string
	CalendarID     string
	GranteeDomain  string
	GranteeSubject string
	Role           string
	CreatedBy      string
	CreatedAt      time.Time
}

const shareCols = `id, calendar_id, grantee_domain, grantee_subject, role, created_by, created_at`

func scanShare(row interface{ Scan(...any) error }) (Share, error) {
	var sh Share
	var created string
	if err := row.Scan(&sh.ID, &sh.CalendarID, &sh.GranteeDomain, &sh.GranteeSubject, &sh.Role, &sh.CreatedBy, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Share{}, ErrNotFound
		}
		return Share{}, err
	}
	sh.CreatedAt = mustTime(created)
	return sh, nil
}

// PutShare creates the share for (calendar, grantee) or changes its role.
func (s *Store) PutShare(ctx context.Context, sh Share) (Share, error) {
	sh.GranteeDomain = strings.ToLower(sh.GranteeDomain)
	now := FormatTime(s.now())
	_, err := s.DB.ExecContext(ctx, `INSERT INTO calendar_shares
		(id, calendar_id, grantee_domain, grantee_subject, role, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (calendar_id, grantee_domain, grantee_subject) DO UPDATE SET role = excluded.role`,
		NewID(), sh.CalendarID, sh.GranteeDomain, sh.GranteeSubject, sh.Role, sh.CreatedBy, now)
	if err != nil {
		return Share{}, err
	}
	return scanShare(s.DB.QueryRowContext(ctx, `SELECT `+shareCols+` FROM calendar_shares
		WHERE calendar_id = ? AND grantee_domain = ? AND grantee_subject = ?`,
		sh.CalendarID, sh.GranteeDomain, sh.GranteeSubject))
}

// GetShare reads one share.
func (s *Store) GetShare(ctx context.Context, id string) (Share, error) {
	return scanShare(s.DB.QueryRowContext(ctx, `SELECT `+shareCols+` FROM calendar_shares WHERE id = ?`, id))
}

// ListShares returns the shares of a calendar.
func (s *Store) ListShares(ctx context.Context, calendarID string) ([]Share, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+shareCols+` FROM calendar_shares WHERE calendar_id = ?
		ORDER BY created_at, id`, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Share
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// ShareRole returns the role of the share for an identity on a calendar,
// or "" when there is none.
func (s *Store) ShareRole(ctx context.Context, calendarID, domain, subject string) (string, error) {
	var role string
	err := s.DB.QueryRowContext(ctx, `SELECT role FROM calendar_shares
		WHERE calendar_id = ? AND grantee_domain = ? AND grantee_subject = ?`,
		calendarID, strings.ToLower(domain), subject).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// DeleteShare removes a share.
func (s *Store) DeleteShare(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM calendar_shares WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
