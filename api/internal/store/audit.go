package store

import (
	"context"
	"database/sql"
	"time"
)

// AuditRow is one audit_log row.
type AuditRow struct {
	ID          string
	At          time.Time
	ActorID     string
	PrincipalID string
	Via         string
	Application string
	Operation   string
	Target      string
	Outcome     string
}

// InsertAudit appends an audit entry.
func (s *Store) InsertAudit(ctx context.Context, a AuditRow) error {
	if a.ID == "" {
		a.ID = NewID()
	}
	if a.At.IsZero() {
		a.At = s.now()
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO audit_log
		(id, at, actor_id, principal_id, via, application, operation, target, outcome)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, FormatTime(a.At), a.ActorID, a.PrincipalID, a.Via, nullStr(a.Application), a.Operation, nullStr(a.Target), a.Outcome)
	return err
}

// ListAudit returns entries where the user is the principal or the actor,
// newest first. delegatedOnly keeps only entries where somebody else (a
// delegate or an application) used the user's authority.
func (s *Store) ListAudit(ctx context.Context, userID string, delegatedOnly bool, limit int) ([]AuditRow, error) {
	q := `SELECT id, at, actor_id, principal_id, via, application, operation, target, outcome FROM audit_log `
	var args []any
	if delegatedOnly {
		q += `WHERE principal_id = ? AND via <> 'self' `
		args = append(args, userID)
	} else {
		q += `WHERE principal_id = ? OR actor_id = ? `
		args = append(args, userID, userID)
	}
	q += `ORDER BY at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var a AuditRow
		var at string
		var app, target sql.NullString
		if err := rows.Scan(&a.ID, &at, &a.ActorID, &a.PrincipalID, &a.Via, &app, &a.Operation, &target, &a.Outcome); err != nil {
			return nil, err
		}
		a.At, a.Application, a.Target = mustTime(at), strOf(app), strOf(target)
		out = append(out, a)
	}
	return out, rows.Err()
}

// RecordActAsNonce stores a presentation nonce until expiresAt. A nonce
// that is already held gives ErrConflict (a replay). Expired nonces are
// purged first.
func (s *Store) RecordActAsNonce(ctx context.Context, nonce string, expiresAt time.Time) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM act_as_nonces WHERE expires_at <= ?`, FormatTime(s.now())); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO act_as_nonces (nonce, expires_at) VALUES (?, ?)`, nonce, FormatTime(expiresAt))
		if isUnique(err) {
			return ErrConflict
		}
		return err
	})
}

// ActAsGrant is an accepted LinkKeys act-as grant.
type ActAsGrant struct {
	GrantID       string
	SubjectDomain string
	UserID        string
	Application   string
	ExpiresAt     time.Time
	RevokedAt     *time.Time
}

// UpsertActAsGrant records a grant catalystcal accepted.
func (s *Store) UpsertActAsGrant(ctx context.Context, g ActAsGrant) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO act_as_grants
		(grant_id, subject_domain, user_id, application, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (grant_id) DO UPDATE SET expires_at = excluded.expires_at`,
		g.GrantID, g.SubjectDomain, g.UserID, g.Application, FormatTime(g.ExpiresAt), FormatTime(s.now()))
	return err
}

// ListLiveActAsGrants returns grants that are not expired and not revoked.
func (s *Store) ListLiveActAsGrants(ctx context.Context) ([]ActAsGrant, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT grant_id, subject_domain, user_id, application, expires_at
		FROM act_as_grants WHERE revoked_at IS NULL AND expires_at > ? ORDER BY subject_domain, grant_id`,
		FormatTime(s.now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActAsGrant
	for rows.Next() {
		var g ActAsGrant
		var exp string
		if err := rows.Scan(&g.GrantID, &g.SubjectDomain, &g.UserID, &g.Application, &exp); err != nil {
			return nil, err
		}
		g.ExpiresAt = mustTime(exp)
		out = append(out, g)
	}
	return out, rows.Err()
}

// MarkActAsGrantsRevoked sets revoked_at on the grants.
func (s *Store) MarkActAsGrantsRevoked(ctx context.Context, grantIDs []string) error {
	if len(grantIDs) == 0 {
		return nil
	}
	args := append([]any{FormatTime(s.now())}, anySlice(grantIDs)...)
	_, err := s.DB.ExecContext(ctx, `UPDATE act_as_grants SET revoked_at = ? WHERE revoked_at IS NULL AND grant_id IN (`+
		placeholders(len(grantIDs))+`)`, args...)
	return err
}

// ListRevokedActAsGrantIDs returns every grant id with a recorded revocation
// that has not expired yet.
func (s *Store) ListRevokedActAsGrantIDs(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT grant_id FROM act_as_grants WHERE revoked_at IS NOT NULL AND expires_at > ?`,
		FormatTime(s.now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
