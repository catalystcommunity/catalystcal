-- +goose Up
-- An application principal is a users row with application_id set: a
-- LinkKeys application of the account (domain, subject). A person has an
-- empty application_id.
ALTER TABLE users ADD COLUMN application_id TEXT NOT NULL DEFAULT '';
DROP INDEX users_identity;
CREATE UNIQUE INDEX users_identity ON users (domain, subject, application_id);

-- A group is a list of identities or other groups of the same owner.
-- revision goes up by one at each change.
CREATE TABLE groups (
    id           TEXT PRIMARY KEY,
    owner_id     TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT,
    revision     INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX groups_owner_name ON groups (owner_id, name);

-- One member: an identity (member_subject set) or a group (member_group_id
-- set), never both.
CREATE TABLE group_members (
    group_id            TEXT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    position            INTEGER NOT NULL,
    member_domain       TEXT NOT NULL DEFAULT '',
    member_subject      TEXT NOT NULL DEFAULT '',
    member_application  TEXT NOT NULL DEFAULT '',
    member_group_id     TEXT REFERENCES groups (id) ON DELETE CASCADE,
    CHECK ((member_group_id IS NULL) = (member_subject <> ''))
);
CREATE INDEX group_members_group ON group_members (group_id, position);
CREATE UNIQUE INDEX group_members_identity ON group_members (group_id, member_domain, member_subject, member_application)
    WHERE member_group_id IS NULL;
CREATE UNIQUE INDEX group_members_nested ON group_members (group_id, member_group_id)
    WHERE member_group_id IS NOT NULL;
CREATE INDEX group_members_lookup ON group_members (member_domain, member_subject, member_application);
CREATE INDEX group_members_parent ON group_members (member_group_id);

-- Shares get the contribute role, application grantees, and group
-- grantees. SQLite cannot change a CHECK, so the table is made again.
CREATE TABLE calendar_shares_new (
    id                   TEXT PRIMARY KEY,
    calendar_id          TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    grantee_domain       TEXT NOT NULL DEFAULT '',
    grantee_subject      TEXT NOT NULL DEFAULT '',
    grantee_application  TEXT NOT NULL DEFAULT '',
    grantee_group_id     TEXT REFERENCES groups (id) ON DELETE CASCADE,
    role                 TEXT NOT NULL CHECK (role IN ('free_busy', 'read', 'contribute', 'write', 'manage')),
    created_by           TEXT NOT NULL REFERENCES users (id),
    created_at           TEXT NOT NULL,
    CHECK ((grantee_group_id IS NULL) = (grantee_subject <> ''))
);
INSERT INTO calendar_shares_new (id, calendar_id, grantee_domain, grantee_subject, role, created_by, created_at)
    SELECT id, calendar_id, grantee_domain, grantee_subject, role, created_by, created_at FROM calendar_shares;
DROP TABLE calendar_shares;
ALTER TABLE calendar_shares_new RENAME TO calendar_shares;
CREATE UNIQUE INDEX calendar_shares_unique ON calendar_shares (calendar_id, grantee_domain, grantee_subject, grantee_application)
    WHERE grantee_group_id IS NULL;
CREATE UNIQUE INDEX calendar_shares_group ON calendar_shares (calendar_id, grantee_group_id)
    WHERE grantee_group_id IS NOT NULL;
CREATE INDEX calendar_shares_grantee ON calendar_shares (grantee_domain, grantee_subject, grantee_application);
CREATE INDEX calendar_shares_grantee_group ON calendar_shares (grantee_group_id);

-- +goose Down
CREATE TABLE calendar_shares_old (
    id                TEXT PRIMARY KEY,
    calendar_id       TEXT NOT NULL REFERENCES calendars (id) ON DELETE CASCADE,
    grantee_domain    TEXT NOT NULL,
    grantee_subject   TEXT NOT NULL,
    role              TEXT NOT NULL CHECK (role IN ('free_busy', 'read', 'write', 'manage')),
    created_by        TEXT NOT NULL REFERENCES users (id),
    created_at        TEXT NOT NULL
);
INSERT INTO calendar_shares_old (id, calendar_id, grantee_domain, grantee_subject, role, created_by, created_at)
    SELECT id, calendar_id, grantee_domain, grantee_subject,
           CASE role WHEN 'contribute' THEN 'read' ELSE role END, created_by, created_at
    FROM calendar_shares WHERE grantee_group_id IS NULL AND grantee_application = '';
DROP TABLE calendar_shares;
ALTER TABLE calendar_shares_old RENAME TO calendar_shares;
CREATE UNIQUE INDEX calendar_shares_unique ON calendar_shares (calendar_id, grantee_domain, grantee_subject);
CREATE INDEX calendar_shares_grantee ON calendar_shares (grantee_domain, grantee_subject);
DROP TABLE group_members;
DROP TABLE groups;
DELETE FROM users WHERE application_id <> '';
DROP INDEX users_identity;
CREATE UNIQUE INDEX users_identity ON users (domain, subject);
ALTER TABLE users DROP COLUMN application_id;
