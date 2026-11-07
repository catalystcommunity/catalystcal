package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/ical"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// maxSplitMappings limits map_recurrence_ids.
const maxSplitMappings = 500

// splitHash identifies the requested content of a split: the event as the
// client sent it, without the receive-only fields, and the recurrence id.
func splitHash(ev csil.Event, rid time.Time) string {
	h := sha256.New()
	h.Write(csil.EncodeEvent(ev))
	h.Write([]byte{0})
	h.Write([]byte(ts(rid)))
	return hex.EncodeToString(h.Sum(nil))
}

func parseMapIDs(in []csil.Timestamp) ([]time.Time, error) {
	if len(in) > maxSplitMappings {
		return nil, badReq("map_recurrence_ids has more than 500 values")
	}
	out := make([]time.Time, len(in))
	for i, v := range in {
		t, err := parseTS("map_recurrence_ids", v)
		if err != nil {
			return nil, err
		}
		out[i] = t
	}
	return out, nil
}

// SplitSeriesOnce makes a this_and_future change at most once for each
// idempotency key. A known key returns its receipt and changes nothing.
func (s *EventService) SplitSeriesOnce(ctx context.Context, req csil.SplitSeriesOnceRequest) (csil.SplitSeriesReceipt, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	if err := checkIdempotencyKey(req.IdempotencyKey); err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	maps, err := parseMapIDs(req.MapRecurrenceIds)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	rid, err := parseTS("recurrence_id", req.RecurrenceId)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	seriesID := string(req.Event.EventId)
	if seriesID == "" {
		return csil.SplitSeriesReceipt{}, badReq("event.event_id is required")
	}
	content := req.Event
	clearReceiveOnly(&content)
	hash := splitHash(content, rid)
	known, err := s.Store.GetSplitReceipt(ctx, seriesID, id.EffectiveUserID(), receiptBinding(id), req.IdempotencyKey)
	switch {
	case err == nil:
		return s.repeatedSplit(ctx, id, r, known, hash, maps)
	case !errors.Is(err, store.ErrNotFound):
		return csil.SplitSeriesReceipt{}, internal(err, "load split receipt")
	}
	prev, c, acc, _, err := r.loadEvent(ctx, seriesID)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+prev.ID())
	if err := s.requireEditable(ctx, id, c, acc, prev); err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	if prev.MasterID() != "" || prev.Ev.Recurrence == nil {
		return csil.SplitSeriesReceipt{}, badReq("event.event_id must be a recurring series")
	}
	if err := validateContent(&content); err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	sp, err := planSplit(id, r, prev, content, rid)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	rc := store.SplitReceipt{
		SeriesID: seriesID, CreatorID: id.EffectiveUserID(), Binding: receiptBinding(id), Key: req.IdempotencyKey,
		CalendarID: c.ID, ContentHash: hash, SeriesCreator: prev.CreatedByID,
		SplitAt: sp.at, InPlace: sp.inPlace, Before: prev.Ev,
	}
	err = s.Store.SplitWithReceipt(ctx, sp.writes, sp.result, &rc)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrReceiptTaken):
		// A split with the same key committed first.
		known, err := s.Store.GetSplitReceipt(ctx, rc.SeriesID, rc.CreatorID, rc.Binding, rc.Key)
		if err != nil {
			return csil.SplitSeriesReceipt{}, internal(err, "load split receipt")
		}
		return s.repeatedSplit(ctx, id, r, known, hash, maps)
	case errors.Is(err, store.ErrConflict):
		return csil.SplitSeriesReceipt{}, csilrpc.Conflict("the series changed at the same time; try again")
	default:
		return csil.SplitSeriesReceipt{}, internal(err, "split series")
	}
	csilrpc.SetAuditTarget(ctx, "event:"+rc.SuccessorID)
	out, err := s.splitOut(ctx, id, r, rc, maps)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	out.Created = true
	return out, nil
}

func (s *EventService) repeatedSplit(ctx context.Context, id *auth.Identity, r *resolver, rc store.SplitReceipt, hash string, maps []time.Time) (csil.SplitSeriesReceipt, error) {
	if err := s.requireSplitAccess(ctx, id, r, rc); err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+rc.SuccessorID)
	if rc.ContentHash != hash {
		return csil.SplitSeriesReceipt{}, csilrpc.Conflict("the idempotency key was used for another split")
	}
	return s.splitOut(ctx, id, r, rc, maps)
}

// requireSplitAccess applies the access check of a split now, on the
// calendar of the split: the caller must be able to change the series.
func (s *EventService) requireSplitAccess(ctx context.Context, id *auth.Identity, r *resolver, rc store.SplitReceipt) error {
	c, acc, err := r.loadCalendar(ctx, rc.CalendarID)
	if err != nil {
		return err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	return s.requireEditable(ctx, id, c, acc, store.Event{CreatedByID: rc.SeriesCreator})
}

// GetSplitReceipt finds the receipt of a split-series-once key.
func (s *EventService) GetSplitReceipt(ctx context.Context, req csil.SplitReceiptRequest) (csil.SplitSeriesReceipt, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	if err := checkIdempotencyKey(req.IdempotencyKey); err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	maps, err := parseMapIDs(req.MapRecurrenceIds)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	rc, err := s.Store.GetSplitReceipt(ctx, string(req.SeriesId), id.EffectiveUserID(), receiptBinding(id), req.IdempotencyKey)
	if errors.Is(err, store.ErrNotFound) {
		return csil.SplitSeriesReceipt{}, errNotFound("split receipt")
	}
	if err != nil {
		return csil.SplitSeriesReceipt{}, internal(err, "load split receipt")
	}
	if err := s.requireSplitAccess(ctx, id, r, rc); err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+rc.SuccessorID)
	return s.splitOut(ctx, id, r, rc, maps)
}

// splitOut reports a split receipt: both series as they are now, and the
// mapping of the requested recurrence ids.
func (s *EventService) splitOut(ctx context.Context, id *auth.Identity, r *resolver, rc store.SplitReceipt, maps []time.Time) (csil.SplitSeriesReceipt, error) {
	out := csil.SplitSeriesReceipt{
		SeriesId: csil.EventID(rc.SeriesID), IdempotencyKey: rc.Key, SuccessorId: csil.EventID(rc.SuccessorID),
		SplitAt: ts(rc.SplitAt), InPlace: rc.InPlace, CreatedAt: ts(rc.CreatedAt), CalendarId: csil.CalendarID(rc.CalendarID),
		Mappings: make([]csil.SplitOccurrenceMapping, 0, len(maps)),
	}
	st, err := s.eventState(ctx, id, r, rc.SeriesID)
	if err != nil {
		return csil.SplitSeriesReceipt{}, err
	}
	out.SeriesState, out.Series, out.SeriesTrashId = st.state, st.event, st.trashID
	if rc.InPlace {
		out.SuccessorState, out.Successor, out.SuccessorTrashId = st.state, st.event, st.trashID
	} else {
		st, err := s.eventState(ctx, id, r, rc.SuccessorID)
		if err != nil {
			return csil.SplitSeriesReceipt{}, err
		}
		out.SuccessorState, out.Successor, out.SuccessorTrashId = st.state, st.event, st.trashID
	}
	for _, m := range maps {
		mp, err := mapSplit(rc, m)
		if err != nil {
			return csil.SplitSeriesReceipt{}, internal(err, "map recurrence id")
		}
		out.Mappings = append(out.Mappings, mp)
	}
	return out, nil
}

// mapSplit maps a recurrence id of the series before the split. It uses
// only the receipt, so the answer does not change after later edits.
func mapSplit(rc store.SplitReceipt, rid time.Time) (csil.SplitOccurrenceMapping, error) {
	out := csil.SplitOccurrenceMapping{RecurrenceId: ts(rid), Result: "none"}
	ok, err := ical.IsInstance(rc.Before, rid)
	if err != nil || !ok {
		return out, err
	}
	if isAllDay(rc.Before) {
		rid = rid.UTC().Truncate(24 * time.Hour)
	}
	if !rc.InPlace && rid.Before(rc.SplitAt) {
		out.Result, out.EventId, out.MappedRecurrenceId = "original", ptr(csil.EventID(rc.SeriesID)), ptr(ts(rid))
		return out, nil
	}
	fromZone, err := seriesZone(rc.Before)
	if err != nil {
		return out, err
	}
	toZone, err := seriesZone(rc.After)
	if err != nil {
		return out, err
	}
	newStart, _, err := store.Bounds(rc.After)
	if err != nil {
		return out, err
	}
	shift := wallClock(newStart, toZone).Sub(wallClock(rc.SplitAt, fromZone))
	mapped := fromWallClock(wallClock(rid, fromZone).Add(shift), toZone)
	if isAllDay(rc.After) {
		mapped = mapped.Truncate(24 * time.Hour)
	}
	ok, err = ical.IsInstance(rc.After, mapped)
	if err != nil || !ok {
		return out, err
	}
	out.Result, out.EventId, out.MappedRecurrenceId = "successor", ptr(csil.EventID(rc.SuccessorID)), ptr(ts(mapped))
	return out, nil
}

func isAllDay(ev csil.Event) bool { return ev.AllDay != nil && *ev.AllDay }

// seriesZone is the zone that a series expands in: its time_zone, or UTC
// for an all-day series and a series without one.
func seriesZone(ev csil.Event) (*time.Location, error) {
	if isAllDay(ev) || ev.Timezone == nil || *ev.Timezone == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(*ev.Timezone)
}

// wallClock is the local time of t in loc, written as a UTC time, so that
// a difference of two wall clocks has no daylight-time change in it.
func wallClock(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), 0, time.UTC)
}

func fromWallClock(w time.Time, loc *time.Location) time.Time {
	return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), 0, loc).UTC()
}
