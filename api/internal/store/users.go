package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// User is one person who signed in to this server.
type User struct {
	ID          string
	Domain      string
	Subject     string
	Handle      string
	DisplayName string
	Email       string
	Timezone    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const userCols = `id, domain, subject, handle, display_name, email, timezone, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var handle, name, email, tz sql.NullString
	var created, updated string
	if err := row.Scan(&u.ID, &u.Domain, &u.Subject, &handle, &name, &email, &tz, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.Handle, u.DisplayName, u.Email, u.Timezone = strOf(handle), strOf(name), strOf(email), strOf(tz)
	u.CreatedAt, u.UpdatedAt = mustTime(created), mustTime(updated)
	return u, nil
}

// UserInput carries identity fields from a sign-in. Empty fields do not
// overwrite stored values.
type UserInput struct {
	Domain      string
	Subject     string
	Handle      string
	DisplayName string
	Email       string
}

// DefaultCalendarName returns the name of a new user's default calendar.
func DefaultCalendarName(displayName string) string {
	if strings.TrimSpace(displayName) == "" {
		return "Calendar"
	}
	return displayName + "'s calendar"
}

// UpsertUser finds or creates the user for a LinkKeys identity and updates
// the claim fields. A new user gets a default calendar in the same
// transaction. created is true for a new user.
func (s *Store) UpsertUser(ctx context.Context, in UserInput) (u User, created bool, err error) {
	in.Domain = strings.ToLower(strings.TrimSpace(in.Domain))
	if in.Domain == "" || in.Subject == "" {
		return User{}, false, errors.New("store: user needs a domain and a subject")
	}
	now := FormatTime(s.now())
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		existing, err := scanUser(tx.QueryRowContext(ctx,
			`SELECT `+userCols+` FROM users WHERE domain = ? AND subject = ?`, in.Domain, in.Subject))
		switch {
		case err == nil:
			_, err = tx.ExecContext(ctx, `UPDATE users SET
				handle = COALESCE(?, handle),
				display_name = COALESCE(?, display_name),
				email = COALESCE(?, email),
				updated_at = ?, last_seen_at = ?
				WHERE id = ?`,
				nullStr(in.Handle), nullStr(in.DisplayName), nullStr(in.Email), now, now, existing.ID)
			if err != nil {
				return err
			}
			u, err = scanUser(tx.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, existing.ID))
			return err
		case errors.Is(err, ErrNotFound):
			id := NewID()
			if _, err := tx.ExecContext(ctx, `INSERT INTO users
				(id, domain, subject, handle, display_name, email, created_at, updated_at, last_seen_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, in.Domain, in.Subject, nullStr(in.Handle), nullStr(in.DisplayName), nullStr(in.Email), now, now, now); err != nil {
				return err
			}
			if _, err := insertCalendar(ctx, tx, Calendar{
				OwnerID:   id,
				Name:      DefaultCalendarName(in.DisplayName),
				Kind:      "local",
				IsDefault: true,
			}, s.now()); err != nil {
				return err
			}
			created = true
			u, err = scanUser(tx.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
			return err
		default:
			return err
		}
	})
	return u, created, err
}

// GetUser reads one user by id.
func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// GetUsers reads several users by id into a map. Unknown ids are skipped.
func (s *Store) GetUsers(ctx context.Context, ids []string) (map[string]User, error) {
	out := map[string]User{}
	uniq := map[string]bool{}
	var args []any
	for _, id := range ids {
		if id != "" && !uniq[id] {
			uniq[id] = true
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE id IN (`+placeholders(len(args))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out[u.ID] = u
	}
	return out, rows.Err()
}

// GetUserByIdentity reads the user for a LinkKeys identity.
func (s *Store) GetUserByIdentity(ctx context.Context, domain, subject string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE domain = ? AND subject = ?`,
		strings.ToLower(domain), subject))
}

// GetUserByHandle finds a user by handle at a domain.
func (s *Store) GetUserByHandle(ctx context.Context, domain, handle string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE domain = ? AND handle = ? ORDER BY created_at LIMIT 1`,
		strings.ToLower(domain), handle))
}

// ListUsers returns every user, oldest first.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateProfile changes the fields a person sets for themselves. A nil
// pointer keeps the value; an empty string clears it.
func (s *Store) UpdateProfile(ctx context.Context, id string, displayName, timezone *string) (User, error) {
	sets := []string{"updated_at = ?"}
	args := []any{FormatTime(s.now())}
	if displayName != nil {
		sets = append(sets, "display_name = ?")
		args = append(args, nullStr(*displayName))
	}
	if timezone != nil {
		sets = append(sets, "timezone = ?")
		args = append(args, nullStr(*timezone))
	}
	args = append(args, id)
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return User{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return User{}, ErrNotFound
	}
	return s.GetUser(ctx, id)
}
