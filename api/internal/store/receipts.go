package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// ErrReceiptTaken: a create receipt with the same key is already stored.
var ErrReceiptTaken = errors.New("store: the idempotency key is already used")

// CreateReceipt is one event_create_receipts row.
type CreateReceipt struct {
	CalendarID string
	CreatorID  string
	// Binding is "" for a self bearer, "app:<application>" for an act-as
	// bearer, "delegate:<user id>" for a delegated bearer.
	Binding     string
	Key         string
	ContentHash string
	EventID     string
	CreatedAt   time.Time
}

// CreateEventWithReceipt stores the receipt and the event in one
// transaction. A receipt with the same key gives ErrReceiptTaken, a uid
// that is taken gives ErrConflict; then nothing is written.
func (s *Store) CreateEventWithReceipt(ctx context.Context, e *Event, r *CreateReceipt) error {
	if e.Ev.EventId == "" {
		e.Ev.EventId = csil.EventID(NewID())
	}
	r.EventID = string(e.Ev.EventId)
	r.CreatedAt = s.now()
	return s.InTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO event_create_receipts (calendar_id, creator_id, binding,
			idempotency_key, content_hash, event_id, created_at) VALUES (`+placeholders(7)+`)`,
			r.CalendarID, r.CreatorID, r.Binding, r.Key, r.ContentHash, r.EventID, FormatTime(r.CreatedAt))
		if isUnique(err) {
			return ErrReceiptTaken
		}
		if err != nil {
			return err
		}
		return s.insertEvent(ctx, tx, e)
	})
}

// GetCreateReceipt reads one receipt.
func (s *Store) GetCreateReceipt(ctx context.Context, calendarID, creatorID, binding, key string) (CreateReceipt, error) {
	r := CreateReceipt{CalendarID: calendarID, CreatorID: creatorID, Binding: binding, Key: key}
	var created string
	err := s.DB.QueryRowContext(ctx, `SELECT content_hash, event_id, created_at FROM event_create_receipts
		WHERE calendar_id = ? AND creator_id = ? AND binding = ? AND idempotency_key = ?`,
		calendarID, creatorID, binding, key).Scan(&r.ContentHash, &r.EventID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return CreateReceipt{}, ErrNotFound
	}
	if err != nil {
		return CreateReceipt{}, err
	}
	r.CreatedAt, _ = ParseTime(created)
	return r, nil
}

// TrashForEvent returns the newest entry of scope all for an event that
// has not expired.
func (s *Store) TrashForEvent(ctx context.Context, eventID string) (TrashEntry, error) {
	return scanTrash(s.DB.QueryRowContext(ctx, `SELECT `+trashCols+` FROM event_trash
		WHERE event_id = ? AND scope = 'all' AND expires_at > ? ORDER BY deleted_at DESC, id DESC LIMIT 1`,
		eventID, FormatTime(s.now())))
}
