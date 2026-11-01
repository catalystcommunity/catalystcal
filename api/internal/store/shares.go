package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Principal names a LinkKeys identity: a person (Application "") or an
// application principal of the account.
type Principal struct {
	Domain      string
	Subject     string
	Application string
}

// Norm lower-cases the domain.
func (p Principal) Norm() Principal {
	p.Domain = strings.ToLower(strings.TrimSpace(p.Domain))
	return p
}

// Share gives a grantee a role on one calendar. The grantee is an identity
// (GranteeSubject set) or a group (GranteeGroupID set).
type Share struct {
	ID                 string
	CalendarID         string
	GranteeDomain      string
	GranteeSubject     string
	GranteeApplication string
	GranteeGroupID     string
	Role               string
	CreatedBy          string
	CreatedAt          time.Time
}

// Grantee returns the identity grantee.
func (sh Share) Grantee() Principal {
	return Principal{Domain: sh.GranteeDomain, Subject: sh.GranteeSubject, Application: sh.GranteeApplication}
}

const shareCols = `id, calendar_id, grantee_domain, grantee_subject, grantee_application, grantee_group_id, role, created_by, created_at`

func scanShare(row interface{ Scan(...any) error }) (Share, error) {
	var sh Share
	var group sql.NullString
	var created string
	if err := row.Scan(&sh.ID, &sh.CalendarID, &sh.GranteeDomain, &sh.GranteeSubject, &sh.GranteeApplication, &group,
		&sh.Role, &sh.CreatedBy, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Share{}, ErrNotFound
		}
		return Share{}, err
	}
	sh.GranteeGroupID = strOf(group)
	sh.CreatedAt = mustTime(created)
	return sh, nil
}

// PutShare creates the share for (calendar, grantee) or changes its role.
func (s *Store) PutShare(ctx context.Context, sh Share) (Share, error) {
	now := FormatTime(s.now())
	if sh.GranteeGroupID != "" {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO calendar_shares
			(id, calendar_id, grantee_group_id, role, created_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (calendar_id, grantee_group_id) WHERE grantee_group_id IS NOT NULL
			DO UPDATE SET role = excluded.role`,
			NewID(), sh.CalendarID, sh.GranteeGroupID, sh.Role, sh.CreatedBy, now)
		if err != nil {
			return Share{}, err
		}
		return scanShare(s.DB.QueryRowContext(ctx, `SELECT `+shareCols+` FROM calendar_shares
			WHERE calendar_id = ? AND grantee_group_id = ?`, sh.CalendarID, sh.GranteeGroupID))
	}
	sh.GranteeDomain = strings.ToLower(sh.GranteeDomain)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO calendar_shares
		(id, calendar_id, grantee_domain, grantee_subject, grantee_application, role, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (calendar_id, grantee_domain, grantee_subject, grantee_application) WHERE grantee_group_id IS NULL
		DO UPDATE SET role = excluded.role`,
		NewID(), sh.CalendarID, sh.GranteeDomain, sh.GranteeSubject, sh.GranteeApplication, sh.Role, sh.CreatedBy, now)
	if err != nil {
		return Share{}, err
	}
	return scanShare(s.DB.QueryRowContext(ctx, `SELECT `+shareCols+` FROM calendar_shares
		WHERE calendar_id = ? AND grantee_domain = ? AND grantee_subject = ? AND grantee_application = ? AND grantee_group_id IS NULL`,
		sh.CalendarID, sh.GranteeDomain, sh.GranteeSubject, sh.GranteeApplication))
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

// granteeMatch is the WHERE part that matches the shares of an identity
// and of the groups that hold it. It takes the identity, then the group
// ids.
func granteeMatch(alias string, p Principal, groupIDs []string) (string, []any) {
	cond := `(` + alias + `grantee_group_id IS NULL AND ` + alias + `grantee_domain = ? AND ` +
		alias + `grantee_subject = ? AND ` + alias + `grantee_application = ?`
	args := []any{p.Domain, p.Subject, p.Application}
	if len(groupIDs) > 0 {
		cond += ` OR ` + alias + `grantee_group_id IN (` + placeholders(len(groupIDs)) + `)`
		for _, g := range groupIDs {
			args = append(args, g)
		}
	}
	return cond + `)`, args
}

// ShareRoles returns the roles of the shares on a calendar for an identity,
// directly or through one of groupIDs (the groups that hold the identity).
// The caller takes the highest.
func (s *Store) ShareRoles(ctx context.Context, calendarID string, p Principal, groupIDs []string) ([]string, error) {
	cond, args := granteeMatch("", p.Norm(), groupIDs)
	rows, err := s.DB.QueryContext(ctx, `SELECT role FROM calendar_shares WHERE calendar_id = ? AND `+cond,
		append([]any{calendarID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
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
