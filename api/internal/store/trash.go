package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// Restore conflicts. A uid or an id that is taken again is ErrConflict.
var (
	// ErrSeriesGone: the series of a this or this_and_future entry is no
	// longer on the calendar (deleted or moved).
	ErrSeriesGone = errors.New("store: the series is no longer on the calendar")
	// ErrSeriesChanged: the series changed after the delete in a way that
	// restore cannot reverse.
	ErrSeriesChanged = errors.New("store: the series changed after the delete")
	// ErrInstanceChanged: the instance has a new override.
	ErrInstanceChanged = errors.New("store: the instance changed after the delete")
)

// Trash scopes. They match the CSIL RecurrenceScope words.
const (
	TrashAll           = "all"
	TrashThis          = "this"
	TrashThisAndFuture = "this_and_future"
)

// TrashSnapshot is what restore needs.
type TrashSnapshot struct {
	// Master is the single event or the series master before the delete.
	Master Event `json:"master"`
	// Removed are the rows that the delete removed, a master before its
	// overrides.
	Removed []Event `json:"removed,omitempty"`
}

// TrashEntry is one event_trash row.
type TrashEntry struct {
	ID           string
	CalendarID   string
	EventID      string
	Scope        string
	RecurrenceID *time.Time
	CreatedBy    string
	DeletedBy    string
	// MasterSequence is the master sequence after a this_and_future delete.
	MasterSequence *int64
	Snapshot       TrashSnapshot
	DeletedAt      time.Time
	ExpiresAt      time.Time
}

const trashCols = `id, calendar_id, event_id, scope, recurrence_id, created_by, deleted_by, master_sequence,
	snapshot, deleted_at, expires_at`

func scanTrash(row interface{ Scan(...any) error }) (TrashEntry, error) {
	var t TrashEntry
	var rid, cby, dby sql.NullString
	var seq sql.NullInt64
	var snap, deleted, expires string
	if err := row.Scan(&t.ID, &t.CalendarID, &t.EventID, &t.Scope, &rid, &cby, &dby, &seq, &snap, &deleted, &expires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TrashEntry{}, ErrNotFound
		}
		return TrashEntry{}, err
	}
	t.RecurrenceID = timeOf(rid)
	t.CreatedBy, t.DeletedBy = strOf(cby), strOf(dby)
	if seq.Valid {
		t.MasterSequence = &seq.Int64
	}
	if err := json.Unmarshal([]byte(snap), &t.Snapshot); err != nil {
		return TrashEntry{}, err
	}
	t.DeletedAt, _ = ParseTime(deleted)
	t.ExpiresAt, _ = ParseTime(expires)
	return t, nil
}

// TrashEvent applies the writes of a delete and records the trash entry,
// in one transaction. It sets the id and the times on t.
func (s *Store) TrashEvent(ctx context.Context, t *TrashEntry, retention time.Duration, writes []EventWrite) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	t.DeletedAt = s.now()
	t.ExpiresAt = t.DeletedAt.Add(retention)
	snap, err := json.Marshal(t.Snapshot)
	if err != nil {
		return err
	}
	var seq any
	if t.MasterSequence != nil {
		seq = *t.MasterSequence
	}
	return s.InTx(ctx, func(tx *sql.Tx) error {
		if err := s.applyEventWrites(ctx, tx, writes); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO event_trash (`+trashCols+`) VALUES (`+placeholders(11)+`)`,
			t.ID, t.CalendarID, t.EventID, t.Scope, nullTime(t.RecurrenceID), nullStr(t.CreatedBy), nullStr(t.DeletedBy),
			seq, string(snap), FormatTime(t.DeletedAt), FormatTime(t.ExpiresAt))
		return err
	})
}

// GetTrash reads one entry that has not expired.
func (s *Store) GetTrash(ctx context.Context, id string) (TrashEntry, error) {
	return scanTrash(s.DB.QueryRowContext(ctx, `SELECT `+trashCols+` FROM event_trash WHERE id = ? AND expires_at > ?`,
		id, FormatTime(s.now())))
}

// ListTrash returns the entries of a calendar that have not expired,
// newest first. A createdBy other than "" keeps the entries of the events
// that user created.
func (s *Store) ListTrash(ctx context.Context, calendarID, createdBy string, limit int) ([]TrashEntry, error) {
	q := `SELECT ` + trashCols + ` FROM event_trash WHERE calendar_id = ? AND expires_at > ?`
	args := []any{calendarID, FormatTime(s.now())}
	if createdBy != "" {
		q += ` AND created_by = ?`
		args = append(args, createdBy)
	}
	q += ` ORDER BY deleted_at DESC, id DESC LIMIT ?`
	rows, err := s.DB.QueryContext(ctx, q, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashEntry
	for rows.Next() {
		t, err := scanTrash(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// PurgeTrash removes the expired entries.
func (s *Store) PurgeTrash(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM event_trash WHERE expires_at <= ?`, FormatTime(s.now()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RestoreTrash reverses the delete of an entry and removes the entry, in
// one transaction. by and via record who restored it. It returns the id
// of the single event or the series master. A conflict is ErrConflict (a
// uid or an id is taken again), ErrSeriesGone, ErrSeriesChanged, or
// ErrInstanceChanged.
func (s *Store) RestoreTrash(ctx context.Context, id, by string, via csil.ActingVia) (string, error) {
	var eventID string
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		t, err := scanTrash(tx.QueryRowContext(ctx, `SELECT `+trashCols+` FROM event_trash WHERE id = ? AND expires_at > ?`,
			id, FormatTime(s.now())))
		if err != nil {
			return err
		}
		eventID = t.EventID
		if t.Scope != TrashAll {
			if err := s.restoreSeries(ctx, tx, t, by, via); err != nil {
				return err
			}
		}
		for i := range t.Snapshot.Removed {
			e := &t.Snapshot.Removed[i]
			e.KeepCreatedAt = true
			if err := s.insertEvent(ctx, tx, e); err != nil {
				if errors.Is(err, ErrConflict) && t.Scope != TrashAll {
					return ErrInstanceChanged
				}
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM event_trash WHERE id = ?`, id)
		return err
	})
	return eventID, err
}

// restoreSeries changes the master back for a this or this_and_future
// entry. The removed overrides are inserted after it.
func (s *Store) restoreSeries(ctx context.Context, tx *sql.Tx, t TrashEntry, by string, via csil.ActingVia) error {
	evs, err := s.loadEvents(ctx, tx, `SELECT `+eventCols+` FROM events WHERE id = ?`, t.EventID)
	if err != nil {
		return err
	}
	if len(evs) == 0 || string(evs[0].Ev.CalendarId) != t.CalendarID || evs[0].Ev.Recurrence == nil || t.RecurrenceID == nil {
		return ErrSeriesGone
	}
	cur := evs[0]
	rec := *cur.Ev.Recurrence
	switch t.Scope {
	case TrashThis:
		rid := csil.Timestamp(FormatTime(*t.RecurrenceID))
		var kept []csil.Timestamp
		found := false
		for _, x := range rec.Exdates {
			if x == rid {
				found = true
				continue
			}
			kept = append(kept, x)
		}
		if !found {
			return ErrSeriesChanged
		}
		rec.Exdates = kept
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE master_id = ? AND recurrence_id = ?`,
			cur.ID(), string(rid)).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrInstanceChanged
		}
	case TrashThisAndFuture:
		if t.MasterSequence == nil || cur.Ev.Sequence != *t.MasterSequence || t.Snapshot.Master.Ev.Recurrence == nil {
			return ErrSeriesChanged
		}
		rec = *t.Snapshot.Master.Ev.Recurrence
		cur.SeriesUntil = t.Snapshot.Master.SeriesUntil
	default:
		return errors.New("store: unknown trash scope " + t.Scope)
	}
	cur.Ev.Recurrence = &rec
	cur.Ev.Sequence++
	cur.Ev.UpdatedVia = &via
	cur.UpdatedByID = by
	return s.updateEvent(ctx, tx, &cur)
}
