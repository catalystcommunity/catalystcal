package services

import (
	"context"
	"sort"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
)

// FreeBusyService implements csil.FreeBusyService.
type FreeBusyService struct{ *Deps }

var _ csil.FreeBusyService = (*FreeBusyService)(nil)

// MaxFreeBusyPeople caps one query.
const MaxFreeBusyPeople = 50

type busy struct {
	start, end time.Time
	tentative  bool
}

// Query returns, for each person, the union of opaque, non-cancelled
// instances on the person's calendars that the caller may see as
// free_busy or better. A person with no such calendar is visible=false.
func (s *FreeBusyService) Query(ctx context.Context, req csil.FreeBusyRequest) (csil.FreeBusyResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.FreeBusyResponse{}, err
	}
	if len(req.People) == 0 {
		return csil.FreeBusyResponse{}, badReq("people must not be empty")
	}
	if len(req.People) > MaxFreeBusyPeople {
		return csil.FreeBusyResponse{}, csilrpc.BadRequest("at most 50 people per query")
	}
	start, end, err := parseRange(req.RangeStart, req.RangeEnd)
	if err != nil {
		return csil.FreeBusyResponse{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.FreeBusyResponse{}, err
	}
	out := csil.FreeBusyResponse{People: []csil.PersonFreeBusy{}}
	for _, ref := range req.People {
		pf := csil.PersonFreeBusy{Identity: ref, Busy: []csil.BusyInterval{}}
		_, _, u, err := s.resolveRef(ctx, ref)
		if err != nil {
			if e, ok := err.(*csilrpc.Error); ok && e.Code == 404 {
				out.People = append(out.People, pf)
				continue
			}
			return csil.FreeBusyResponse{}, err
		}
		if u == nil {
			out.People = append(out.People, pf)
			continue
		}
		p := person(*u)
		pf.Person = &p
		cals, err := s.Store.ListCalendarsByOwner(ctx, u.ID)
		if err != nil {
			return csil.FreeBusyResponse{}, internal(err, "list calendars")
		}
		var ids []string
		for _, c := range cals {
			acc, err := r.calendar(ctx, c)
			if err != nil {
				return csil.FreeBusyResponse{}, err
			}
			if acc.Role >= access.FreeBusy {
				ids = append(ids, c.ID)
			}
		}
		if len(ids) == 0 {
			out.People = append(out.People, pf)
			continue
		}
		pf.Visible = true
		rows, err := s.Store.ListEventsInRange(ctx, ids, start, end)
		if err != nil {
			return csil.FreeBusyResponse{}, internal(err, "list events")
		}
		masters, overrides := groupRows(rows)
		var spans []busy
		remaining := MaxOccurrences
		for _, m := range masters {
			insts, _, err := expandRow(m, overrides[m.ID()], start, end, remaining)
			if err != nil {
				return csil.FreeBusyResponse{}, internal(err, "expand series")
			}
			remaining -= len(insts)
			for _, in := range insts {
				ev := in.Event
				if ev.Status != nil && *ev.Status == "cancelled" {
					continue
				}
				if ev.Transparency != nil && *ev.Transparency == "transparent" {
					continue
				}
				st, en := in.Start, in.End
				if st.Before(start) {
					st = start
				}
				if en.After(end) {
					en = end
				}
				spans = append(spans, busy{st, en, ev.Status != nil && *ev.Status == "tentative"})
			}
			if remaining <= 0 {
				break
			}
		}
		for _, b := range mergeBusy(spans) {
			bi := csil.BusyInterval{Start: ts(b.start), End: ts(b.end)}
			if b.tentative {
				bi.Tentative = ptr(true)
			}
			pf.Busy = append(pf.Busy, bi)
		}
		out.People = append(out.People, pf)
	}
	return out, nil
}

// mergeBusy unions overlapping spans. A merged span is tentative only when
// every part of it is tentative.
func mergeBusy(in []busy) []busy {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool { return in[i].start.Before(in[j].start) })
	out := []busy{in[0]}
	for _, b := range in[1:] {
		last := &out[len(out)-1]
		if !b.start.After(last.end) {
			if b.end.After(last.end) {
				last.end = b.end
			}
			last.tentative = last.tentative && b.tentative
			continue
		}
		out = append(out, b)
	}
	return out
}
