package services

import (
	"context"
	"errors"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// DefaultTrashRetention applies when the configuration has none (tests).
const DefaultTrashRetention = 30 * 24 * time.Hour

func (d *Deps) trashRetention() time.Duration {
	if d.Cfg == nil || d.Cfg.TrashRetention <= 0 {
		return DefaultTrashRetention
	}
	return d.Cfg.TrashRetention
}

// trashCreatorFilter returns the creator filter of the trash a caller may
// see on a calendar: "" for every entry (write access or more), the
// effective user for contribute access. ok is false when the caller can
// delete no event there, so the caller sees no trash.
func trashCreatorFilter(id *auth.Identity, c store.Calendar, acc access.Access) (createdBy string, ok bool) {
	if c.Kind == "subscription" {
		return "", false
	}
	if acc.CanWrite() {
		return "", true
	}
	if acc.CanCreate() {
		return id.EffectiveUserID(), true
	}
	return "", false
}

// ListTrash returns the trash entries of one calendar that the caller can
// restore: the delete rule decides.
func (s *EventService) ListTrash(ctx context.Context, req csil.ListTrashRequest) (csil.TrashList, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.TrashList{}, err
	}
	c, acc, err := r.loadCalendar(ctx, string(req.CalendarId))
	if err != nil {
		return csil.TrashList{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	out := csil.TrashList{Entries: []csil.TrashEntry{}}
	createdBy, ok := trashCreatorFilter(id, c, acc)
	if !ok {
		if c.Kind == "subscription" {
			return out, nil
		}
		return csil.TrashList{}, csilrpc.Forbidden("you cannot delete or restore events on this calendar")
	}
	limit := 100
	if req.Limit != nil && *req.Limit > 0 {
		limit = int(min(*req.Limit, 500))
	}
	rows, err := s.Store.ListTrash(ctx, c.ID, createdBy, limit)
	if err != nil {
		return csil.TrashList{}, internal(err, "list trash")
	}
	pp := s.people()
	for _, t := range rows {
		e, err := s.trashOut(ctx, pp, id, r, acc, t)
		if err != nil {
			return csil.TrashList{}, err
		}
		out.Entries = append(out.Entries, e)
	}
	return out, nil
}

func (s *EventService) trashOut(ctx context.Context, pp *people, id *auth.Identity, r *resolver, acc access.Access, t store.TrashEntry) (csil.TrashEntry, error) {
	m := t.Snapshot.Master
	view := access.ViewEvent(id, acc, access.IsPrivate(m.Ev.Visibility), r.isAttendee(m.Ev))
	ev, err := pp.viewOut(ctx, m, view)
	if err != nil {
		return csil.TrashEntry{}, err
	}
	out := csil.TrashEntry{
		TrashId: csil.TrashID(t.ID), CalendarId: csil.CalendarID(t.CalendarID), EventId: csil.EventID(t.EventID),
		Scope: csil.RecurrenceScope(t.Scope), RecurrenceId: tsPtr(t.RecurrenceID), Event: ev,
		DeletedAt: ts(t.DeletedAt), ExpiresAt: ts(t.ExpiresAt),
	}
	if t.Scope == store.TrashThis && t.RecurrenceID != nil {
		inst := m
		if len(t.Snapshot.Removed) > 0 {
			inst = t.Snapshot.Removed[0]
		} else if content, err := instanceContent(m.Ev, *t.RecurrenceID); err == nil {
			inst.Ev = content
		}
		iv, err := pp.viewOut(ctx, inst, view)
		if err != nil {
			return csil.TrashEntry{}, err
		}
		out.Instance = &iv
	}
	if out.DeletedBy, err = pp.id(ctx, t.DeletedBy); err != nil {
		return csil.TrashEntry{}, internal(err, "load deleted_by")
	}
	return out, nil
}

// RestoreEvent reverses one delete. The caller needs the right to delete
// the event now: write access or more, or contribute access for an event
// that the caller created.
func (s *EventService) RestoreEvent(ctx context.Context, req csil.RestoreEventRequest) (csil.Event, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.Event{}, err
	}
	notFound := errNotFound("trash entry")
	t, err := s.Store.GetTrash(ctx, string(req.TrashId))
	if errors.Is(err, store.ErrNotFound) {
		return csil.Event{}, notFound
	}
	if err != nil {
		return csil.Event{}, internal(err, "load trash entry")
	}
	c, acc, err := r.loadCalendar(ctx, t.CalendarID)
	if err != nil {
		return csil.Event{}, notFound
	}
	csilrpc.SetAuditTarget(ctx, "event:"+t.EventID)
	createdBy, ok := trashCreatorFilter(id, c, acc)
	if !ok {
		return csil.Event{}, notFound
	}
	if createdBy != "" && createdBy != t.CreatedBy {
		return csil.Event{}, csilrpc.Forbidden("you can restore only the events that you created on this calendar")
	}
	eventID, err := s.Store.RestoreTrash(ctx, t.ID, id.EffectiveUserID(), *viaOf(id))
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		return csil.Event{}, notFound
	case errors.Is(err, store.ErrConflict):
		return csil.Event{}, csilrpc.Conflict("an event with the same uid is on the calendar again")
	case errors.Is(err, store.ErrSeriesGone):
		return csil.Event{}, csilrpc.Conflict("the series is no longer on this calendar; restore the series first")
	case errors.Is(err, store.ErrSeriesChanged):
		return csil.Event{}, csilrpc.Conflict("the series changed after the delete")
	case errors.Is(err, store.ErrInstanceChanged):
		return csil.Event{}, csilrpc.Conflict("the instance changed after the delete")
	default:
		return csil.Event{}, internal(err, "restore event")
	}
	return s.reload(ctx, eventID)
}
