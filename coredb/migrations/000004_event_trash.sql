-- +goose Up
-- One delete that restore can reverse. The delete removes the rows from
-- events as before, so no read path sees a deleted event. The snapshot
-- holds what restore needs, as JSON: the series master or single event as
-- it was before the delete, and the rows that the delete removed.
--   all             - the event or the series and its overrides
--   this            - one instance: an EXDATE on the master, and its
--                     override if it had one
--   this_and_future - the master was truncated; the overrides at and
--                     after the instance were removed
CREATE TABLE event_trash (
    id                TEXT PRIMARY KEY,
    calendar_id       TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    -- The single event or the series master.
    event_id          TEXT NOT NULL,
    scope             TEXT NOT NULL CHECK (scope IN ('all', 'this', 'this_and_future')),
    recurrence_id     TEXT,
    -- The creator of the event or series, for the contribute rule.
    created_by        TEXT REFERENCES users (id) ON DELETE SET NULL,
    deleted_by        TEXT REFERENCES users (id) ON DELETE SET NULL,
    -- The master sequence after a this_and_future delete. Restore refuses
    -- when the master changed after the delete.
    master_sequence   INTEGER,
    snapshot          TEXT NOT NULL,
    deleted_at        TEXT NOT NULL,
    expires_at        TEXT NOT NULL
);
CREATE INDEX event_trash_calendar ON event_trash (calendar_id, deleted_at);
CREATE INDEX event_trash_expires ON event_trash (expires_at);

-- +goose Down
DROP TABLE event_trash;
