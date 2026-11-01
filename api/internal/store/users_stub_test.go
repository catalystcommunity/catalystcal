package store

import (
	"context"
	"testing"
)

func TestEnsureUserMakesAStubUntilSignIn(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := Principal{Domain: "Dev.Local", Subject: "frank"}
	u, err := s.EnsureUser(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.EnsureUser(ctx, p)
	if err != nil || again.ID != u.ID || u.Domain != "dev.local" {
		t.Fatalf("ensure twice: %+v %+v %v", u, again, err)
	}
	if cals, _ := s.ListCalendarsByOwner(ctx, u.ID); len(cals) != 0 {
		t.Fatalf("stub has calendars: %d", len(cals))
	}
	if all, _ := s.ListUsers(ctx); len(all) != 0 {
		t.Fatalf("stub listed: %+v", all)
	}
	signed, created, err := s.UpsertUser(ctx, UserInput{Domain: "dev.local", Subject: "frank", Handle: "frank", DisplayName: "Frank"})
	if err != nil || created || signed.ID != u.ID {
		t.Fatalf("sign-in: %+v created=%v %v", signed, created, err)
	}
	if cals, _ := s.ListCalendarsByOwner(ctx, u.ID); len(cals) != 1 || !cals[0].IsDefault {
		t.Fatalf("calendars after sign-in: %+v", cals)
	}
	// A second sign-in adds no calendar.
	if _, _, err := s.UpsertUser(ctx, UserInput{Domain: "dev.local", Subject: "frank"}); err != nil {
		t.Fatal(err)
	}
	if cals, _ := s.ListCalendarsByOwner(ctx, u.ID); len(cals) != 1 {
		t.Fatalf("calendars after second sign-in: %d", len(cals))
	}
	if all, _ := s.ListUsers(ctx); len(all) != 1 {
		t.Fatalf("listed after sign-in: %d", len(all))
	}
}
