package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// Migration 3 makes calendar_shares again. A 0.1.x database keeps its users
// and shares.
func TestMigration3KeepsShares(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := gooseSetup(); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, db, "migrations", 2); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users (id, domain, subject, created_at, updated_at) VALUES ('u1', 'd', 'alice', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		`INSERT INTO calendars (id, owner_id, name, created_at, updated_at) VALUES ('c1', 'u1', 'A', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		`INSERT INTO calendar_shares (id, calendar_id, grantee_domain, grantee_subject, role, created_by, created_at)
			VALUES ('s1', 'c1', 'd', 'bob', 'write', 'u1', '2026-01-01T00:00:00Z')`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	sh, err := s.GetShare(ctx, "s1")
	if err != nil || sh.Role != "write" || sh.GranteeSubject != "bob" || sh.GranteeApplication != "" || sh.GranteeGroupID != "" {
		t.Fatalf("share after migration: %+v %v", sh, err)
	}
	u, err := s.GetUserByIdentity(ctx, "d", "alice")
	if err != nil || u.ID != "u1" || u.ApplicationID != "" {
		t.Fatalf("user after migration: %+v %v", u, err)
	}
	roles, err := s.ShareRoles(ctx, "c1", Principal{Domain: "d", Subject: "bob"}, nil)
	if err != nil || len(roles) != 1 || roles[0] != "write" {
		t.Fatalf("roles %v %v", roles, err)
	}
	// Down and up again works.
	if err := goose.DownToContext(ctx, db, "migrations", 2); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
}
