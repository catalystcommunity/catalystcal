package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"unicode/utf8"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// receiptBinding scopes a create receipt to the bearer: the exact grantee
// application of an act-as bearer, the delegate of a delegated bearer.
func receiptBinding(id *auth.Identity) string {
	switch id.Via {
	case auth.ViaActAsGrant:
		if id.Grantee != "" {
			return "app:" + id.Grantee
		}
		return "app:" + id.Application
	case auth.ViaDelegation:
		return "delegate:" + id.UserID
	}
	return ""
}

func checkIdempotencyKey(k string) error {
	if n := utf8.RuneCountInString(k); n < 1 || n > 200 {
		return badReq("idempotency_key must have 1 to 200 characters")
	}
	return nil
}

// contentHash identifies the requested content of a create: the event as
// the client sent it, without the receive-only fields.
func contentHash(ev csil.Event) string {
	sum := sha256.Sum256(csil.EncodeEvent(ev))
	return hex.EncodeToString(sum[:])
}

// CreateEventOnce makes an event at most once for each idempotency key.
// A known key returns its receipt and makes nothing.
func (s *EventService) CreateEventOnce(ctx context.Context, req csil.CreateEventOnceRequest) (csil.CreateEventReceipt, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	if err := checkIdempotencyKey(req.IdempotencyKey); err != nil {
		return csil.CreateEventReceipt{}, err
	}
	ev := req.Event
	clearReceiveOnly(&ev)
	if ev.CalendarId == "" {
		return csil.CreateEventReceipt{}, badReq("event.calendar_id is required")
	}
	c, acc, err := r.loadCalendar(ctx, string(ev.CalendarId))
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if err := requireCreatable(c, acc); err != nil {
		return csil.CreateEventReceipt{}, err
	}
	rc := store.CreateReceipt{
		CalendarID: c.ID, CreatorID: id.EffectiveUserID(), Binding: receiptBinding(id),
		Key: req.IdempotencyKey, ContentHash: contentHash(ev),
	}
	known, err := s.Store.GetCreateReceipt(ctx, rc.CalendarID, rc.CreatorID, rc.Binding, rc.Key)
	switch {
	case err == nil:
		return s.repeatedCreate(ctx, id, r, known, rc.ContentHash)
	case !errors.Is(err, store.ErrNotFound):
		return csil.CreateEventReceipt{}, internal(err, "load create receipt")
	}
	row, err := newEventRow(id, r, ev)
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	err = s.Store.CreateEventWithReceipt(ctx, &row, &rc)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrReceiptTaken):
		// A create with the same key committed first.
		known, err := s.Store.GetCreateReceipt(ctx, rc.CalendarID, rc.CreatorID, rc.Binding, rc.Key)
		if err != nil {
			return csil.CreateEventReceipt{}, internal(err, "load create receipt")
		}
		return s.repeatedCreate(ctx, id, r, known, rc.ContentHash)
	case errors.Is(err, store.ErrConflict):
		return csil.CreateEventReceipt{}, csilrpc.Conflict("an event with this uid is already on the calendar")
	default:
		return csil.CreateEventReceipt{}, internal(err, "create event")
	}
	csilrpc.SetAuditTarget(ctx, "event:"+row.ID())
	out, err := s.receiptOut(ctx, id, r, rc)
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	out.Created = true
	return out, nil
}

func (s *EventService) repeatedCreate(ctx context.Context, id *auth.Identity, r *resolver, rc store.CreateReceipt, hash string) (csil.CreateEventReceipt, error) {
	csilrpc.SetAuditTarget(ctx, "event:"+rc.EventID)
	if rc.ContentHash != hash {
		return csil.CreateEventReceipt{}, csilrpc.Conflict("the idempotency key was used for other event content")
	}
	return s.receiptOut(ctx, id, r, rc)
}

// GetCreateReceipt finds the receipt of a create-event-once key.
func (s *EventService) GetCreateReceipt(ctx context.Context, req csil.CreateReceiptRequest) (csil.CreateEventReceipt, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	if err := checkIdempotencyKey(req.IdempotencyKey); err != nil {
		return csil.CreateEventReceipt{}, err
	}
	c, acc, err := r.loadCalendar(ctx, string(req.CalendarId))
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if err := requireCreatable(c, acc); err != nil {
		return csil.CreateEventReceipt{}, err
	}
	rc, err := s.Store.GetCreateReceipt(ctx, c.ID, id.EffectiveUserID(), receiptBinding(id), req.IdempotencyKey)
	if errors.Is(err, store.ErrNotFound) {
		return csil.CreateEventReceipt{}, errNotFound("create receipt")
	}
	if err != nil {
		return csil.CreateEventReceipt{}, internal(err, "load create receipt")
	}
	csilrpc.SetAuditTarget(ctx, "event:"+rc.EventID)
	return s.receiptOut(ctx, id, r, rc)
}

// receiptOut reports the event of a receipt as it is now. The caller's
// access now decides what it shows.
func (s *EventService) receiptOut(ctx context.Context, id *auth.Identity, r *resolver, rc store.CreateReceipt) (csil.CreateEventReceipt, error) {
	out := csil.CreateEventReceipt{
		CalendarId: csil.CalendarID(rc.CalendarID), IdempotencyKey: rc.Key, EventId: csil.EventID(rc.EventID),
		CreatedAt: ts(rc.CreatedAt),
	}
	st, err := s.eventState(ctx, id, r, rc.EventID)
	if err != nil {
		return csil.CreateEventReceipt{}, err
	}
	out.State, out.Event, out.TrashId = st.state, st.event, st.trashID
	return out, nil
}

// receiptEvent is the state of an event that a receipt names.
type receiptEvent struct {
	state   csil.CreateReceiptState
	event   *csil.Event
	trashID *csil.TrashID
}

// eventState finds an event that a receipt names: on a calendar (live or
// hidden), in trash, or gone. The caller's access now decides what it
// shows.
func (s *EventService) eventState(ctx context.Context, id *auth.Identity, r *resolver, eventID string) (receiptEvent, error) {
	e, err := s.Store.GetEvent(ctx, eventID)
	switch {
	case err == nil:
		c, err := s.Store.GetCalendar(ctx, string(e.Ev.CalendarId))
		if err != nil {
			return receiptEvent{}, internal(err, "load event calendar")
		}
		acc, err := r.calendar(ctx, c)
		if err != nil {
			return receiptEvent{}, err
		}
		view := access.ViewEvent(id, acc, access.IsPrivate(e.Ev.Visibility), r.isAttendee(e.Ev))
		if !view.CanSee() {
			return receiptEvent{state: "hidden"}, nil
		}
		ev, err := s.people().viewOut(ctx, e, view)
		if err != nil {
			return receiptEvent{}, err
		}
		return receiptEvent{state: "live", event: &ev}, nil
	case !errors.Is(err, store.ErrNotFound):
		return receiptEvent{}, internal(err, "load event")
	}
	t, err := s.Store.TrashForEvent(ctx, eventID)
	if errors.Is(err, store.ErrNotFound) {
		return receiptEvent{state: "deleted"}, nil
	}
	if err != nil {
		return receiptEvent{}, internal(err, "load trash entry")
	}
	out := receiptEvent{state: "trashed"}
	if tc, tacc, err := r.loadCalendar(ctx, t.CalendarID); err == nil {
		if createdBy, ok := trashCreatorFilter(id, tc, tacc); ok && (createdBy == "" || createdBy == t.CreatedBy) {
			out.trashID = ptr(csil.TrashID(t.ID))
		}
	}
	return out, nil
}
