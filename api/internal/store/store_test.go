package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if p, err := Pending(context.Background(), db); err != nil || p {
		t.Fatalf("pending after migrate: %v %v", p, err)
	}
	return New(db)
}

func TestTimeHelpers(t *testing.T) {
	ts := time.Date(2026, 10, 15, 17, 0, 0, 0, time.FixedZone("x", -6*3600))
	if FormatTime(ts) != "2026-10-15T23:00:00Z" {
		t.Fatal(FormatTime(ts))
	}
	got, err := ParseTime("2026-10-15T17:00:00-06:00")
	if err != nil || FormatTime(got) != "2026-10-15T23:00:00Z" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := ParseTime("yesterday"); err == nil {
		t.Fatal("bad time parsed")
	}
	if NewID() == NewID() {
		t.Fatal("ids repeat")
	}
}

func TestUpsertUserMakesDefaultCalendar(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, created, err := s.UpsertUser(ctx, UserInput{Domain: "Example.COM", Subject: "S1", DisplayName: "Ann"})
	if err != nil || !created || u.Domain != "example.com" {
		t.Fatalf("%+v %v %v", u, created, err)
	}
	cals, _ := s.ListCalendarsByOwner(ctx, u.ID)
	if len(cals) != 1 || !cals[0].IsDefault || cals[0].Name != "Ann's calendar" {
		t.Fatalf("calendars: %+v", cals)
	}
	again, created, err := s.UpsertUser(ctx, UserInput{Domain: "example.com", Subject: "S1", Email: "a@example.com"})
	if err != nil || created || again.ID != u.ID || again.DisplayName != "Ann" || again.Email != "a@example.com" {
		t.Fatalf("second upsert: %+v %v %v", again, created, err)
	}
	cals, _ = s.ListCalendarsByOwner(ctx, u.ID)
	if len(cals) != 1 {
		t.Fatal("second default calendar")
	}
}

func TestSessionRotationGraceAndReuse(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _, _ := s.UpsertUser(ctx, UserInput{Domain: "d", Subject: "s"})
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	if _, err := s.CreateSession(ctx, u.ID, "h1", "ua", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RotateSession(ctx, "h1", "h2", time.Hour, time.Minute); err != nil {
		t.Fatal(err)
	}
	// The old secret inside the grace window: no rotation, no revocation.
	if _, err := s.RotateSession(ctx, "h1", "h3", time.Hour, time.Minute); !errors.Is(err, ErrSessionRecentlyRotated) {
		t.Fatalf("grace: %v", err)
	}
	// After the window: reuse revokes the session.
	now = now.Add(2 * time.Minute)
	if _, err := s.RotateSession(ctx, "h1", "h4", time.Hour, time.Minute); !errors.Is(err, ErrSessionReuse) {
		t.Fatalf("reuse: %v", err)
	}
	if _, err := s.RotateSession(ctx, "h2", "h5", time.Hour, time.Minute); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("after reuse: %v", err)
	}
}

func TestActAsNonceReplay(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	exp := time.Now().Add(time.Minute)
	if err := s.RecordActAsNonce(ctx, "n1", exp); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordActAsNonce(ctx, "n1", exp); !errors.Is(err, ErrConflict) {
		t.Fatalf("replay: %v", err)
	}
}

func TestDelegationLiveIndex(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _, _ := s.UpsertUser(ctx, UserInput{Domain: "d", Subject: "p"})
	d := Delegation{PrincipalID: u.ID, DelegateDomain: "D", DelegateSubject: "x", Scopes: []string{"events.write"}}
	if _, err := s.CreateDelegation(ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDelegation(ctx, d); !errors.Is(err, ErrConflict) {
		t.Fatalf("second live delegation: %v", err)
	}
	got, err := s.FindLiveDelegation(ctx, u.ID, "d", "x")
	if err != nil || got.DelegateDomain != "d" {
		t.Fatalf("%+v %v", got, err)
	}
}
