-- +goose Up
-- Browser session rotation keeps the previous token hash for a short grace
-- window. Two tabs that resume at the same moment then both succeed, and a
-- reuse after the window revokes the session.
ALTER TABLE sessions ADD COLUMN previous_token_hash TEXT;
ALTER TABLE sessions ADD COLUMN rotated_at TEXT;
CREATE INDEX sessions_previous_token ON sessions (previous_token_hash);

-- LinkKeys act-as grants that catalystcal accepted (catalystcal is the audience). The
-- revocation worker asks the RP about each live grant. revoked_at is set
-- when a verified revocation arrives; the dispatcher then refuses every
-- bearer minted from the grant.
CREATE TABLE act_as_grants (
    grant_id         TEXT PRIMARY KEY,
    subject_domain   TEXT NOT NULL,
    user_id          TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    application      TEXT NOT NULL,
    expires_at       TEXT NOT NULL,
    revoked_at       TEXT,
    created_at       TEXT NOT NULL,
    last_checked_at  TEXT
);
CREATE INDEX act_as_grants_expiry ON act_as_grants (expires_at);

-- +goose Down
DROP TABLE act_as_grants;
DROP INDEX sessions_previous_token;
ALTER TABLE sessions DROP COLUMN rotated_at;
ALTER TABLE sessions DROP COLUMN previous_token_hash;
