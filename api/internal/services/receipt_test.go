package services

import (
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// These digests came from the text-based schema and csilgen 0.2.10.
func TestReceiptHashesSurviveDateUpgrade(t *testing.T) {
	at := time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC)
	ev := csil.Event{CalendarId: "cal", Title: "Leap day", AllDay: ptr(true), StartDate: ptr("2000-02-29"),
		EndDate: ptr("2000-03-01"), Start: &at, Recurrence: &csil.Recurrence{Rrule: "FREQ=YEARLY", Exdates: []time.Time{at}}}
	for _, tc := range []struct {
		splitAt *time.Time
		want    string
	}{
		{nil, "ad154a69dc1a86dc25be539ce23449d1479bec3328ff6a5f640fa9457570eaf6"},
		{&at, "79c085c843742637282d3342d06c12d3f7f9ec189f49d0a7b8c5ef31da8ec030"},
	} {
		got, err := contentHash(ev, tc.splitAt)
		if err != nil || got != tc.want {
			t.Errorf("hash = %q, %v; want %q", got, err, tc.want)
		}
	}
}
