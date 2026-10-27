package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Session is a browser session. Only the SHA-256 of its secret is stored.
type Session struct {
	ID         string
	UserID     string
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

var (
	// ErrSessionInvalid means the session is unknown, revoked, or expired.
	ErrSessionInvalid = errors.New("store: session is invalid")
	// ErrSessionReuse means an old secret came back after the grace window.
	// The session is revoked.
	ErrSessionReuse = errors.New("store: session secret reuse")
	// ErrSessionRecentlyRotated means an old secret came back inside the
	// grace window (two tabs resumed at once). The caller mints a bearer but
	// does not rotate again.
	ErrSessionRecentlyRotated = errors.New("store: session rotated a moment ago")
)

const sessionCols = `id, user_id, created_at, last_used_at, expires_at, revoked_at`

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var ss Session
	var created, used, expires string
	var revoked sql.NullString
	if err := row.Scan(&ss.ID, &ss.UserID, &created, &used, &expires, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	ss.CreatedAt, ss.LastUsedAt, ss.ExpiresAt = mustTime(created), mustTime(used), mustTime(expires)
	ss.RevokedAt = timeOf(revoked)
	return ss, nil
}

// CreateSession stores a new browser session.
func (s *Store) CreateSession(ctx context.Context, userID, tokenHash, userAgent string, lifetime time.Duration) (Session, error) {
	now := s.now()
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions
		(id, user_id, token_hash, user_agent, created_at, last_used_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, userID, tokenHash, nullStr(userAgent), FormatTime(now), FormatTime(now), FormatTime(now.Add(lifetime)))
	if err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, id)
}

// GetSession reads a session by id.
func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	return scanSession(s.DB.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions WHERE id = ?`, id))
}

// RotateSession swaps the session secret and slides the expiry. A secret
// that was rotated less than grace ago returns ErrSessionRecentlyRotated
// with the session; an older rotated secret revokes the session and
// returns ErrSessionReuse.
func (s *Store) RotateSession(ctx context.Context, oldHash, newHash string, lifetime, grace time.Duration) (Session, error) {
	now := s.now()
	var out Session
	var outErr error
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		ss, err := scanSession(tx.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions WHERE token_hash = ?`, oldHash))
		if errors.Is(err, ErrNotFound) {
			var rotated sql.NullString
			prev, perr := scanSessionWithRotated(tx.QueryRowContext(ctx,
				`SELECT `+sessionCols+`, rotated_at FROM sessions WHERE previous_token_hash = ?`, oldHash), &rotated)
			if errors.Is(perr, ErrNotFound) {
				outErr = ErrSessionInvalid
				return nil
			}
			if perr != nil {
				return perr
			}
			if !live(prev, now) {
				outErr = ErrSessionInvalid
				return nil
			}
			if r := timeOf(rotated); r != nil && now.Sub(*r) <= grace {
				out, outErr = prev, ErrSessionRecentlyRotated
				return nil
			}
			if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ?`, FormatTime(now), prev.ID); err != nil {
				return err
			}
			outErr = ErrSessionReuse
			return nil
		}
		if err != nil {
			return err
		}
		if !live(ss, now) {
			outErr = ErrSessionInvalid
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET token_hash = ?, previous_token_hash = ?, rotated_at = ?,
			last_used_at = ?, expires_at = ? WHERE id = ?`,
			newHash, oldHash, FormatTime(now), FormatTime(now), FormatTime(now.Add(lifetime)), ss.ID); err != nil {
			return err
		}
		out, err = scanSession(tx.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions WHERE id = ?`, ss.ID))
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return out, outErr
}

func scanSessionWithRotated(row *sql.Row, rotated *sql.NullString) (Session, error) {
	var ss Session
	var created, used, expires string
	var revoked sql.NullString
	if err := row.Scan(&ss.ID, &ss.UserID, &created, &used, &expires, &revoked, rotated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	ss.CreatedAt, ss.LastUsedAt, ss.ExpiresAt = mustTime(created), mustTime(used), mustTime(expires)
	ss.RevokedAt = timeOf(revoked)
	return ss, nil
}

func live(ss Session, now time.Time) bool {
	return ss.RevokedAt == nil && ss.ExpiresAt.After(now)
}

// RevokeSessionByHash ends the session that holds the secret (current or
// previous). It returns ErrSessionInvalid when no live session matches.
func (s *Store) RevokeSessionByHash(ctx context.Context, tokenHash string) (Session, error) {
	now := s.now()
	ss, err := scanSession(s.DB.QueryRowContext(ctx,
		`SELECT `+sessionCols+` FROM sessions WHERE token_hash = ? OR previous_token_hash = ? LIMIT 1`, tokenHash, tokenHash))
	if errors.Is(err, ErrNotFound) {
		return Session{}, ErrSessionInvalid
	}
	if err != nil {
		return Session{}, err
	}
	if !live(ss, now) {
		return ss, ErrSessionInvalid
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ?`, FormatTime(now), ss.ID); err != nil {
		return Session{}, err
	}
	ss.RevokedAt = &now
	return ss, nil
}
