-- +goose Up
-- catalystcal baseline schema (SQLite).
--
-- Time columns are TEXT in one fixed layout: 2006-01-02T15:04:05Z (UTC).
-- Text comparison and ORDER BY then work as time comparison.
-- IDs are ULIDs made in Go.
-- Foreign keys need `_pragma=foreign_keys(1)` in the DSN.

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    domain        TEXT NOT NULL,
    subject       TEXT NOT NULL,
    handle        TEXT,
    display_name  TEXT,
    email         TEXT,
    timezone      TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    last_seen_at  TEXT
);
-- Domains compare case-insensitively; the store lower-cases them on write.
CREATE UNIQUE INDEX users_identity ON users (domain, subject);
CREATE INDEX users_handle ON users (domain, handle);

-- A browser session. The refresh secret is in an httpOnly cookie; only its
-- SHA-256 is stored.
CREATE TABLE sessions (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash    TEXT NOT NULL UNIQUE,
    user_agent    TEXT,
    created_at    TEXT NOT NULL,
    last_used_at  TEXT NOT NULL,
    expires_at    TEXT NOT NULL,
    revoked_at    TEXT
);
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE calendars (
    id              TEXT PRIMARY KEY,
    owner_id        TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    color           TEXT,
    timezone        TEXT,
    kind            TEXT NOT NULL DEFAULT 'local' CHECK (kind IN ('local', 'subscription')),
    source_url      TEXT,
    last_synced_at  TEXT,
    sync_error      TEXT,
    -- The secret part of the public iCalendar feed URL. NULL = feed off.
    feed_token      TEXT UNIQUE,
    is_default      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX calendars_owner ON calendars (owner_id);
-- One default calendar for each owner.
CREATE UNIQUE INDEX calendars_one_default ON calendars (owner_id) WHERE is_default = 1;

-- A share gives another person a role on one calendar. That person acts
-- under their own name. The grantee is a LinkKeys identity, so a share can
-- exist before the grantee first signs in.
CREATE TABLE calendar_shares (
    id                TEXT PRIMARY KEY,
    calendar_id       TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    grantee_domain    TEXT NOT NULL,
    grantee_subject   TEXT NOT NULL,
    role              TEXT NOT NULL CHECK (role IN ('free_busy', 'read', 'write', 'manage')),
    created_by        TEXT NOT NULL REFERENCES users (id),
    created_at        TEXT NOT NULL
);
CREATE UNIQUE INDEX calendar_shares_unique ON calendar_shares (calendar_id, grantee_domain, grantee_subject);
CREATE INDEX calendar_shares_grantee ON calendar_shares (grantee_domain, grantee_subject);

-- A delegation lets the delegate act AS the principal, within the scopes.
CREATE TABLE delegations (
    id                TEXT PRIMARY KEY,
    principal_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    delegate_domain   TEXT NOT NULL,
    delegate_subject  TEXT NOT NULL,
    -- Space-separated Scope words.
    scopes            TEXT NOT NULL,
    note              TEXT,
    expires_at        TEXT,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    revoked_at        TEXT
);
CREATE INDEX delegations_principal ON delegations (principal_id);
CREATE INDEX delegations_delegate ON delegations (delegate_domain, delegate_subject);
-- At most one live delegation for each (principal, delegate) pair.
CREATE UNIQUE INDEX delegations_live ON delegations (principal_id, delegate_domain, delegate_subject)
    WHERE revoked_at IS NULL;

-- Calendars a delegation is limited to. No rows = every calendar of the
-- principal.
CREATE TABLE delegation_calendars (
    delegation_id  TEXT NOT NULL REFERENCES delegations (id) ON DELETE CASCADE,
    calendar_id    TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    PRIMARY KEY (delegation_id, calendar_id)
);

-- An event row is a single event, a series master (rrule set), or an
-- override of one instance of a series (master_id + recurrence_id set).
-- Series are expanded in Go when read; they are not materialized, so an
-- RRULE round-trips to iCalendar unchanged.
CREATE TABLE events (
    id                TEXT PRIMARY KEY,
    calendar_id       TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    uid               TEXT NOT NULL,
    master_id         TEXT REFERENCES events (id) ON DELETE CASCADE,
    -- For an override: the original start of the instance it replaces.
    recurrence_id     TEXT,
    title             TEXT NOT NULL,
    description       TEXT,
    location          TEXT,
    url               TEXT,
    conference_url    TEXT,
    -- UTC instants. For an all-day event: 00:00:00Z of start_date and of
    -- the exclusive end_date, so range queries work for both kinds.
    starts_at         TEXT NOT NULL,
    ends_at           TEXT NOT NULL,
    all_day           INTEGER NOT NULL DEFAULT 0,
    start_date        TEXT,
    end_date          TEXT,
    timezone          TEXT,
    status            TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed', 'tentative', 'cancelled')),
    transparency      TEXT NOT NULL DEFAULT 'opaque' CHECK (transparency IN ('opaque', 'transparent')),
    visibility        TEXT NOT NULL DEFAULT 'default' CHECK (visibility IN ('default', 'public', 'private', 'confidential')),
    rrule             TEXT,
    -- JSON arrays of UTC instants.
    exdates           TEXT,
    rdates            TEXT,
    -- The end of the series: the last possible instance end, or NULL for an
    -- open series. Lets a range query skip finished series.
    series_until      TEXT,
    categories        TEXT,
    color             TEXT,
    organizer_id      TEXT REFERENCES users (id),
    created_by        TEXT REFERENCES users (id),
    updated_by        TEXT REFERENCES users (id),
    updated_via       TEXT NOT NULL DEFAULT 'self' CHECK (updated_via IN ('self', 'delegation', 'act_as_grant')),
    sequence          INTEGER NOT NULL DEFAULT 0,
    -- Lower-cased title + location + description, folded in Go.
    search_text       TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX events_calendar_time ON events (calendar_id, starts_at);
CREATE INDEX events_master ON events (master_id);
CREATE UNIQUE INDEX events_override ON events (master_id, recurrence_id) WHERE master_id IS NOT NULL;
-- One master (or single event) per UID per calendar.
CREATE UNIQUE INDEX events_uid ON events (calendar_id, uid) WHERE master_id IS NULL;

CREATE TABLE event_attendees (
    id                TEXT PRIMARY KEY,
    event_id          TEXT NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    -- A catalystcal user by LinkKeys identity, or an email address, or both.
    identity_domain   TEXT,
    identity_subject  TEXT,
    email             TEXT,
    name              TEXT,
    role              TEXT NOT NULL DEFAULT 'required' CHECK (role IN ('chair', 'required', 'optional', 'non_participant')),
    status            TEXT NOT NULL DEFAULT 'needs_action' CHECK (status IN ('needs_action', 'accepted', 'declined', 'tentative', 'delegated')),
    rsvp              INTEGER NOT NULL DEFAULT 1,
    -- Set when a delegate answered for this attendee.
    responded_by      TEXT REFERENCES users (id),
    responded_at      TEXT,
    position          INTEGER NOT NULL DEFAULT 0,
    CHECK (identity_subject IS NOT NULL OR email IS NOT NULL)
);
CREATE INDEX event_attendees_event ON event_attendees (event_id);
CREATE INDEX event_attendees_identity ON event_attendees (identity_domain, identity_subject);

CREATE TABLE event_reminders (
    event_id        TEXT NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    minutes_before  INTEGER NOT NULL,
    method          TEXT NOT NULL DEFAULT 'display' CHECK (method IN ('display', 'email')),
    PRIMARY KEY (event_id, minutes_before, method)
);

CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,
    at              TEXT NOT NULL,
    actor_id        TEXT NOT NULL,
    principal_id    TEXT NOT NULL,
    via             TEXT NOT NULL,
    application     TEXT,
    operation       TEXT NOT NULL,
    target          TEXT,
    outcome         TEXT NOT NULL
);
CREATE INDEX audit_log_principal ON audit_log (principal_id, at);
CREATE INDEX audit_log_actor ON audit_log (actor_id, at);

-- Presentation nonces of accepted LinkKeys act-as credentials (replay
-- protection). Rows expire after the accepted presentation age.
CREATE TABLE act_as_nonces (
    nonce       TEXT PRIMARY KEY,
    expires_at  TEXT NOT NULL
);

-- +goose Down
DROP TABLE act_as_nonces;
DROP TABLE audit_log;
DROP TABLE event_reminders;
DROP TABLE event_attendees;
DROP TABLE events;
DROP TABLE delegation_calendars;
DROP TABLE delegations;
DROP TABLE calendar_shares;
DROP TABLE calendars;
DROP TABLE sessions;
DROP TABLE users;
