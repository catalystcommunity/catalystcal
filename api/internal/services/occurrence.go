package services

import (
	"context"
	"errors"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/ical"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// GetOccurrence returns one occurrence by its stable reference: the series
// and the original start of the instance. An override applies wherever it
// moved the instance.
func (s *EventService) GetOccurrence(ctx context.Context, req csil.GetOccurrenceRequest) (csil.Occurrence, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.Occurrence{}, err
	}
	notFound := errNotFound("occurrence")
	var rid *time.Time
	if req.RecurrenceId != nil {
		t, err := parseTS("recurrence_id", *req.RecurrenceId)
		if err != nil {
			return csil.Occurrence{}, err
		}
		rid = &t
	}
	row, _, acc, _, err := r.loadEvent(ctx, string(req.EventId))
	if err != nil {
		return csil.Occurrence{}, err
	}
	master := row
	if m := row.MasterID(); m != "" {
		// An override row: use its series and its recurrence id.
		if row.Ev.RecurrenceId == nil {
			return csil.Occurrence{}, notFound
		}
		orid, err := parseTS("recurrence_id", *row.Ev.RecurrenceId)
		if err != nil {
			return csil.Occurrence{}, internal(err, "override recurrence id")
		}
		if rid != nil && !rid.Equal(orid) {
			return csil.Occurrence{}, notFound
		}
		rid = &orid
		if master, err = s.Store.GetEvent(ctx, m); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return csil.Occurrence{}, notFound
			}
			return csil.Occurrence{}, internal(err, "load series")
		}
	}
	if master.Ev.Recurrence == nil {
		if rid != nil {
			return csil.Occurrence{}, notFound
		}
		start, end, err := store.Bounds(master.Ev)
		if err != nil {
			return csil.Occurrence{}, internal(err, "event bounds")
		}
		return s.occurrenceOut(ctx, id, r, acc, master, master.Ev, start, end, nil)
	}
	if rid == nil {
		return csil.Occurrence{}, badReq("recurrence_id is required for a recurring series")
	}
	key := rid.UTC()
	if master.Ev.AllDay != nil && *master.Ev.AllDay {
		key = key.Truncate(24 * time.Hour)
	}
	ok, err := ical.IsInstance(master.Ev, key)
	if err != nil {
		return csil.Occurrence{}, internal(err, "expand series "+master.ID())
	}
	if !ok {
		return csil.Occurrence{}, notFound
	}
	ov, err := s.Store.FindOverride(ctx, master.ID(), key)
	switch {
	case err == nil:
		if ov.Ev.Status != nil && *ov.Ev.Status == "cancelled" {
			return csil.Occurrence{}, notFound
		}
		start, end, err := store.Bounds(ov.Ev)
		if err != nil {
			return csil.Occurrence{}, internal(err, "override bounds")
		}
		return s.occurrenceOut(ctx, id, r, acc, ov, ov.Ev, start, end, &key)
	case errors.Is(err, store.ErrNotFound):
		inst, err := instanceContent(master.Ev, key)
		if err != nil {
			return csil.Occurrence{}, internal(err, "instance times")
		}
		start, end, err := store.Bounds(inst)
		if err != nil {
			return csil.Occurrence{}, internal(err, "instance bounds")
		}
		return s.occurrenceOut(ctx, id, r, acc, master, inst, start, end, &key)
	default:
		return csil.Occurrence{}, internal(err, "find override")
	}
}

// occurrenceOut builds an occurrence as list-events does: the row as the
// caller can see it, with the times of the instance.
func (s *EventService) occurrenceOut(ctx context.Context, id *auth.Identity, r *resolver, acc access.Access, row store.Event,
	times csil.Event, start, end time.Time, rid *time.Time) (csil.Occurrence, error) {
	view := access.ViewEvent(id, acc, access.IsPrivate(row.Ev.Visibility), r.isAttendee(row.Ev))
	if !view.CanSee() {
		return csil.Occurrence{}, errNotFound("occurrence")
	}
	ev, err := s.people().viewOut(ctx, row, view)
	if err != nil {
		return csil.Occurrence{}, err
	}
	ev.Start, ev.End, ev.StartDate, ev.EndDate = times.Start, times.End, times.StartDate, times.EndDate
	occ := csil.Occurrence{
		Event: ev, Start: ts(start), End: ts(end),
		Access: view.Role.CSIL(), BusyOnly: view.BusyOnly, Invited: view.Invited,
	}
	if rid != nil {
		occ.RecurrenceId = ptr(ts(*rid))
	}
	return occ, nil
}
