-- +goose Up
-- One receipt for each split-series-once key. The receipt and the split
-- are written in one transaction. The receipt stays after the series
-- change, move, go to trash, or are purged, so a key never splits a
-- second time. series_id and successor_id have no foreign key for this
-- reason.
--   binding         - as in event_create_receipts
--   calendar_id     - the calendar of the series at the split
--   series_creator  - created_by of the series at the split (the
--                     contribute access check)
--   series_before   - canonical CBOR (csil Event) of the series before
--                     the split
--   successor_after - canonical CBOR of the successor as the split wrote
--                     it
--   split_at        - the first instance that the successor took
CREATE TABLE event_split_receipts (
    series_id        TEXT NOT NULL,
    creator_id       TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    binding          TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    calendar_id      TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    -- SHA-256 (hex) of the requested event and recurrence id.
    content_hash     TEXT NOT NULL,
    series_creator   TEXT NOT NULL,
    successor_id     TEXT NOT NULL,
    split_at         TEXT NOT NULL,
    in_place         INTEGER NOT NULL,
    series_before    BLOB NOT NULL,
    successor_after  BLOB NOT NULL,
    created_at       TEXT NOT NULL,
    PRIMARY KEY (series_id, creator_id, binding, idempotency_key)
);

-- +goose Down
DROP TABLE event_split_receipts;
