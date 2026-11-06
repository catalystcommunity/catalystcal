-- +goose Up
-- One receipt for each create-event-once key. The receipt and the event
-- are written in one transaction. The receipt stays after the event
-- changes, moves, goes to trash, or is purged, so a key never makes a
-- second event. event_id has no foreign key for this reason.
--   binding - '' for a self bearer, 'app:<application>' for an act-as
--             bearer, 'delegate:<user id>' for a delegated bearer
CREATE TABLE event_create_receipts (
    calendar_id      TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    creator_id       TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    binding          TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    -- SHA-256 (hex) of the canonical CBOR of the requested event.
    content_hash     TEXT NOT NULL,
    event_id         TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    PRIMARY KEY (calendar_id, creator_id, binding, idempotency_key)
);
-- A receipt finds the trash entry of its event.
CREATE INDEX event_trash_event ON event_trash (event_id);

-- +goose Down
DROP INDEX event_trash_event;
DROP TABLE event_create_receipts;
