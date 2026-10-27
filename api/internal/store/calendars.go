package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Calendar is one calendar row.
type Calendar struct {
	ID           string
	OwnerID      string
	Name         string
	Description  string
	Color        string
	Timezone     string
	Kind         string // local or subscription
	SourceURL    string
	LastSyncedAt *time.Time
	SyncError    string
	FeedToken    string
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const calendarCols = `id, owner_id, name, description, color, timezone, kind, source_url,
	last_synced_at, sync_error, feed_token, is_default, created_at, updated_at`

func scanCalendar(row interface{ Scan(...any) error }) (Calendar, error) {
	var c Calendar
	var desc, color, tz, src, synced, syncErr, feed sql.NullString
	var created, updated string
	if err := row.Scan(&c.ID, &c.OwnerID, &c.Name, &desc, &color, &tz, &c.Kind, &src,
		&synced, &syncErr, &feed, &c.IsDefault, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, ErrNotFound
		}
		return Calendar{}, err
	}
	c.Description, c.Color, c.Timezone, c.SourceURL = strOf(desc), strOf(color), strOf(tz), strOf(src)
	c.LastSyncedAt, c.SyncError, c.FeedToken = timeOf(synced), strOf(syncErr), strOf(feed)
	c.CreatedAt, c.UpdatedAt = mustTime(created), mustTime(updated)
	return c, nil
}

func scanCalendars(rows *sql.Rows, err error) ([]Calendar, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Calendar
	for rows.Next() {
		c, err := scanCalendar(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func insertCalendar(ctx context.Context, q queryer, c Calendar, now time.Time) (string, error) {
	if c.ID == "" {
		c.ID = NewID()
	}
	if c.Kind == "" {
		c.Kind = "local"
	}
	ts := FormatTime(now)
	_, err := q.ExecContext(ctx, `INSERT INTO calendars
		(id, owner_id, name, description, color, timezone, kind, source_url, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.OwnerID, c.Name, nullStr(c.Description), nullStr(c.Color), nullStr(c.Timezone),
		c.Kind, nullStr(c.SourceURL), c.IsDefault, ts, ts)
	return c.ID, err
}

// CreateCalendar stores a new calendar and returns it.
func (s *Store) CreateCalendar(ctx context.Context, c Calendar) (Calendar, error) {
	id, err := insertCalendar(ctx, s.DB, c, s.now())
	if err != nil {
		return Calendar{}, err
	}
	return s.GetCalendar(ctx, id)
}

// GetCalendar reads one calendar.
func (s *Store) GetCalendar(ctx context.Context, id string) (Calendar, error) {
	return scanCalendar(s.DB.QueryRowContext(ctx, `SELECT `+calendarCols+` FROM calendars WHERE id = ?`, id))
}

// GetCalendarByFeedToken finds the calendar of a public feed.
func (s *Store) GetCalendarByFeedToken(ctx context.Context, token string) (Calendar, error) {
	if token == "" {
		return Calendar{}, ErrNotFound
	}
	return scanCalendar(s.DB.QueryRowContext(ctx, `SELECT `+calendarCols+` FROM calendars WHERE feed_token = ?`, token))
}

// ListCalendarsByOwner returns the owner's calendars, default first.
func (s *Store) ListCalendarsByOwner(ctx context.Context, ownerID string) ([]Calendar, error) {
	return scanCalendars(s.DB.QueryContext(ctx, `SELECT `+calendarCols+` FROM calendars WHERE owner_id = ?
		ORDER BY is_default DESC, name, id`, ownerID))
}

// SharedCalendar is a calendar plus the role of a share on it.
type SharedCalendar struct {
	Calendar Calendar
	Role     string
}

// ListCalendarsSharedWith returns calendars with a share for the identity.
func (s *Store) ListCalendarsSharedWith(ctx context.Context, domain, subject string) ([]SharedCalendar, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+prefixCols("c", calendarCols)+`, sh.role
		FROM calendar_shares sh JOIN calendars c ON c.id = sh.calendar_id
		WHERE sh.grantee_domain = ? AND sh.grantee_subject = ?
		ORDER BY c.name, c.id`, strings.ToLower(domain), subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SharedCalendar
	for rows.Next() {
		var sc SharedCalendar
		var c Calendar
		var desc, color, tz, src, synced, syncErr, feed sql.NullString
		var created, updated string
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Name, &desc, &color, &tz, &c.Kind, &src,
			&synced, &syncErr, &feed, &c.IsDefault, &created, &updated, &sc.Role); err != nil {
			return nil, err
		}
		c.Description, c.Color, c.Timezone, c.SourceURL = strOf(desc), strOf(color), strOf(tz), strOf(src)
		c.LastSyncedAt, c.SyncError, c.FeedToken = timeOf(synced), strOf(syncErr), strOf(feed)
		c.CreatedAt, c.UpdatedAt = mustTime(created), mustTime(updated)
		sc.Calendar = c
		out = append(out, sc)
	}
	return out, rows.Err()
}

func prefixCols(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

// CalendarPatch holds the settings UpdateCalendar may change. Nil keeps the
// value; an empty string clears an optional one.
type CalendarPatch struct {
	Name        *string
	Description *string
	Color       *string
	Timezone    *string
}

// UpdateCalendar applies a patch.
func (s *Store) UpdateCalendar(ctx context.Context, id string, p CalendarPatch) (Calendar, error) {
	sets := []string{"updated_at = ?"}
	args := []any{FormatTime(s.now())}
	if p.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *p.Name)
	}
	if p.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, nullStr(*p.Description))
	}
	if p.Color != nil {
		sets = append(sets, "color = ?")
		args = append(args, nullStr(*p.Color))
	}
	if p.Timezone != nil {
		sets = append(sets, "timezone = ?")
		args = append(args, nullStr(*p.Timezone))
	}
	args = append(args, id)
	res, err := s.DB.ExecContext(ctx, `UPDATE calendars SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return Calendar{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Calendar{}, ErrNotFound
	}
	return s.GetCalendar(ctx, id)
}

// DeleteCalendar removes a calendar and, by cascade, its events and shares.
func (s *Store) DeleteCalendar(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM calendars WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFeedToken sets or clears (empty token) the public feed secret.
func (s *Store) SetFeedToken(ctx context.Context, id, token string) (Calendar, error) {
	_, err := s.DB.ExecContext(ctx, `UPDATE calendars SET feed_token = ?, updated_at = ? WHERE id = ?`,
		nullStr(token), FormatTime(s.now()), id)
	if err != nil {
		return Calendar{}, err
	}
	return s.GetCalendar(ctx, id)
}

// ListSubscriptionsDue returns subscription calendars last synced before
// cutoff, or never.
func (s *Store) ListSubscriptionsDue(ctx context.Context, cutoff time.Time) ([]Calendar, error) {
	return scanCalendars(s.DB.QueryContext(ctx, `SELECT `+calendarCols+` FROM calendars
		WHERE kind = 'subscription' AND (last_synced_at IS NULL OR last_synced_at < ?)
		ORDER BY last_synced_at, id`, FormatTime(cutoff)))
}

// SetSyncError records a failed subscription sync.
func (s *Store) SetSyncError(ctx context.Context, id, msg string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE calendars SET sync_error = ?, last_synced_at = ?, updated_at = ? WHERE id = ?`,
		nullStr(msg), FormatTime(s.now()), FormatTime(s.now()), id)
	return err
}
