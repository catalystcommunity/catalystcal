package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// SplitReceipt is one event_split_receipts row.
type SplitReceipt struct {
	SeriesID  string
	CreatorID string
	// Binding is as in CreateReceipt.
	Binding     string
	Key         string
	CalendarID  string
	ContentHash string
	// SeriesCreator is created_by of the series at the split.
	SeriesCreator string
	SuccessorID   string
	SplitAt       time.Time
	InPlace       bool
	// Before is the series before the split; After is the successor as
	// the split wrote it.
	Before    csil.Event
	After     csil.Event
	CreatedAt time.Time
}

// SplitWithReceipt applies the writes of a split and stores the receipt
// in one transaction. After is read from successor when the writes are
// done. A receipt with the same key gives ErrReceiptTaken; then nothing
// is written.
func (s *Store) SplitWithReceipt(ctx context.Context, writes []EventWrite, successor *Event, r *SplitReceipt) error {
	r.CreatedAt = s.now()
	return s.InTx(ctx, func(tx *sql.Tx) error {
		if err := s.applyEventWrites(ctx, tx, writes); err != nil {
			return err
		}
		r.SuccessorID, r.After = successor.ID(), successor.Ev
		inPlace := 0
		if r.InPlace {
			inPlace = 1
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO event_split_receipts (series_id, creator_id, binding,
			idempotency_key, calendar_id, content_hash, series_creator, successor_id, split_at, in_place,
			series_before, successor_after, created_at) VALUES (`+placeholders(13)+`)`,
			r.SeriesID, r.CreatorID, r.Binding, r.Key, r.CalendarID, r.ContentHash, r.SeriesCreator,
			r.SuccessorID, FormatTime(r.SplitAt), inPlace, csil.EncodeEvent(r.Before), csil.EncodeEvent(r.After),
			FormatTime(r.CreatedAt))
		if isUnique(err) {
			return ErrReceiptTaken
		}
		return err
	})
}

// GetSplitReceipt reads one receipt.
func (s *Store) GetSplitReceipt(ctx context.Context, seriesID, creatorID, binding, key string) (SplitReceipt, error) {
	r := SplitReceipt{SeriesID: seriesID, CreatorID: creatorID, Binding: binding, Key: key}
	var splitAt, created string
	var inPlace int
	var before, after []byte
	err := s.DB.QueryRowContext(ctx, `SELECT calendar_id, content_hash, series_creator, successor_id, split_at,
		in_place, series_before, successor_after, created_at FROM event_split_receipts
		WHERE series_id = ? AND creator_id = ? AND binding = ? AND idempotency_key = ?`,
		seriesID, creatorID, binding, key).Scan(&r.CalendarID, &r.ContentHash, &r.SeriesCreator, &r.SuccessorID,
		&splitAt, &inPlace, &before, &after, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return SplitReceipt{}, ErrNotFound
	}
	if err != nil {
		return SplitReceipt{}, err
	}
	if r.Before, err = csil.DecodeEvent(before); err != nil {
		return SplitReceipt{}, err
	}
	if r.After, err = csil.DecodeEvent(after); err != nil {
		return SplitReceipt{}, err
	}
	r.InPlace = inPlace != 0
	r.SplitAt, _ = ParseTime(splitAt)
	r.CreatedAt, _ = ParseTime(created)
	return r, nil
}
