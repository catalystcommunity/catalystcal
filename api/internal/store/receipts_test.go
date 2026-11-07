package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func TestCreateEventWithReceiptOnce(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _, err := s.UpsertUser(ctx, UserInput{Domain: "dev.local", Subject: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	cals, err := s.ListCalendarsByOwner(ctx, u.ID)
	if err != nil || len(cals) != 1 {
		t.Fatalf("calendars %v %v", cals, err)
	}
	cal := cals[0].ID
	newRow := func(uid string) *Event {
		start, end := csil.Timestamp("2026-11-10T15:00:00Z"), csil.Timestamp("2026-11-10T16:00:00Z")
		return &Event{Ev: csil.Event{CalendarId: csil.CalendarID(cal), Uid: &uid, Title: "x", Start: &start, End: &end},
			OrganizerID: u.ID, CreatedByID: u.ID, UpdatedByID: u.ID}
	}
	rc := CreateReceipt{CalendarID: cal, CreatorID: u.ID, Key: "k", ContentHash: "h"}
	first := newRow("a@x")
	if err := s.CreateEventWithReceipt(ctx, first, &rc); err != nil {
		t.Fatal(err)
	}
	// The same key again: refused, and no second event is written.
	again := rc
	second := newRow("b@x")
	if err := s.CreateEventWithReceipt(ctx, second, &again); !errors.Is(err, ErrReceiptTaken) {
		t.Fatalf("second: %v", err)
	}
	if _, err := s.GetEvent(ctx, second.ID()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second event written: %v", err)
	}
	// A uid that is taken: refused, and no receipt is written.
	other := CreateReceipt{CalendarID: cal, CreatorID: u.ID, Key: "k2", ContentHash: "h"}
	if err := s.CreateEventWithReceipt(ctx, newRow("a@x"), &other); !errors.Is(err, ErrConflict) {
		t.Fatalf("uid conflict: %v", err)
	}
	if _, err := s.GetCreateReceipt(ctx, cal, u.ID, "", "k2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("receipt written: %v", err)
	}
	got, err := s.GetCreateReceipt(ctx, cal, u.ID, "", "k")
	if err != nil || got.EventID != first.ID() {
		t.Fatalf("receipt %+v %v", got, err)
	}
	// The receipt stays after the event is deleted.
	if err := s.DeleteEvent(ctx, first.ID()); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetCreateReceipt(ctx, cal, u.ID, "", "k"); err != nil || got.EventID != first.ID() {
		t.Fatalf("receipt after delete %+v %v", got, err)
	}
}

func TestSplitWithReceiptOnce(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _, err := s.UpsertUser(ctx, UserInput{Domain: "dev.local", Subject: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	cals, err := s.ListCalendarsByOwner(ctx, u.ID)
	if err != nil || len(cals) != 1 {
		t.Fatalf("calendars %v %v", cals, err)
	}
	cal := cals[0].ID
	newRow := func(uid string) *Event {
		start, end := csil.Timestamp("2026-11-10T15:00:00Z"), csil.Timestamp("2026-11-10T16:00:00Z")
		return &Event{Ev: csil.Event{EventId: csil.EventID(NewID()), CalendarId: csil.CalendarID(cal), Uid: &uid, Title: "x",
			Start: &start, End: &end, Recurrence: &csil.Recurrence{Rrule: "FREQ=WEEKLY"}},
			OrganizerID: u.ID, CreatedByID: u.ID, UpdatedByID: u.ID}
	}
	rc := SplitReceipt{SeriesID: "series", CreatorID: u.ID, Key: "k", CalendarID: cal, ContentHash: "h", SeriesCreator: u.ID,
		SplitAt: time.Date(2026, 11, 17, 15, 0, 0, 0, time.UTC), Before: newRow("before@x").Ev}
	first := newRow("a@x")
	if err := s.SplitWithReceipt(ctx, []EventWrite{{Insert: first}}, first, &rc); err != nil {
		t.Fatal(err)
	}
	// The same key again: refused, and the writes are rolled back.
	again := rc
	second := newRow("b@x")
	if err := s.SplitWithReceipt(ctx, []EventWrite{{Insert: second}}, second, &again); !errors.Is(err, ErrReceiptTaken) {
		t.Fatalf("second: %v", err)
	}
	if _, err := s.GetEvent(ctx, second.ID()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second split written: %v", err)
	}
	got, err := s.GetSplitReceipt(ctx, "series", u.ID, "", "k")
	if err != nil || got.SuccessorID != first.ID() || !got.SplitAt.Equal(rc.SplitAt) || got.After.Title != "x" ||
		got.After.Recurrence == nil || string(got.Before.EventId) != string(rc.Before.EventId) {
		t.Fatalf("receipt %+v %v", got, err)
	}
}
