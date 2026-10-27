package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// Event is one events row: a single event, a series master, or an override
// of one instance. Ev holds the content in the generated wire type, because
// the ical package and the services work on csil.Event. The person summaries
// (Organizer, CreatedBy, UpdatedBy, Attendee.Person, Attendee.RespondedBy)
// are not stored in Ev; the ids below are.
type Event struct {
	Ev          csil.Event
	OrganizerID string
	CreatedByID string
	UpdatedByID string
	// RespondedBy holds, for each attendee in Ev.Attendees (same index), the
	// user id of a delegate who answered for the attendee, or "".
	RespondedBy []string
	SeriesUntil *time.Time
}

// ID is the event id.
func (e *Event) ID() string { return string(e.Ev.EventId) }

// MasterID is the master id of an override, or "".
func (e *Event) MasterID() string {
	if e.Ev.MasterEventId == nil {
		return ""
	}
	return string(*e.Ev.MasterEventId)
}

// Bounds returns the UTC start and end instants of the row. An all-day
// event uses 00:00:00Z of its dates.
func Bounds(ev csil.Event) (time.Time, time.Time, error) {
	if ev.AllDay != nil && *ev.AllDay {
		if ev.StartDate == nil || ev.EndDate == nil {
			return time.Time{}, time.Time{}, errors.New("all-day event needs start_date and end_date")
		}
		s, err := time.Parse(DateLayout, string(*ev.StartDate))
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		e, err := time.Parse(DateLayout, string(*ev.EndDate))
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return s, e, nil
	}
	if ev.Start == nil || ev.End == nil {
		return time.Time{}, time.Time{}, errors.New("timed event needs start and end")
	}
	s, err := ParseTime(string(*ev.Start))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	e, err := ParseTime(string(*ev.End))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return s, e, nil
}

// SearchText folds the searchable fields in Go (Unicode-correct), so the
// LIKE in SearchEvents compares already-folded bytes.
func SearchText(ev csil.Event) string {
	parts := []string{ev.Title}
	if ev.Location != nil {
		parts = append(parts, *ev.Location)
	}
	if ev.Description != nil {
		parts = append(parts, *ev.Description)
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

const eventCols = `id, calendar_id, uid, master_id, recurrence_id, title, description, location, url, conference_url,
	starts_at, ends_at, all_day, start_date, end_date, timezone, status, transparency, visibility,
	rrule, exdates, rdates, series_until, categories, color, organizer_id, created_by, updated_by,
	updated_via, sequence, created_at, updated_at`

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var e Event
	ev := &e.Ev
	var master, recID, desc, loc, url, conf, startDate, endDate, tz, rrule, exdates, rdates, until, cats, color, org, cby, uby sql.NullString
	var startsAt, endsAt, status, transp, vis, via, created, updated string
	var allDay bool
	var id, cal, uid string
	if err := row.Scan(&id, &cal, &uid, &master, &recID, &ev.Title, &desc, &loc, &url, &conf,
		&startsAt, &endsAt, &allDay, &startDate, &endDate, &tz, &status, &transp, &vis,
		&rrule, &exdates, &rdates, &until, &cats, &color, &org, &cby, &uby,
		&via, &ev.Sequence, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, ErrNotFound
		}
		return Event{}, err
	}
	ev.EventId, ev.CalendarId, ev.Uid = csil.EventID(id), csil.CalendarID(cal), &uid
	if master.Valid {
		m := csil.EventID(master.String)
		ev.MasterEventId = &m
	}
	if recID.Valid {
		r := csil.Timestamp(recID.String)
		ev.RecurrenceId = &r
	}
	ev.Description, ev.Location, ev.Url, ev.ConferenceUrl = optStr(desc), optStr(loc), optStr(url), optStr(conf)
	ev.AllDay = &allDay
	if allDay {
		sd, ed := csil.DateText(strOf(startDate)), csil.DateText(strOf(endDate))
		ev.StartDate, ev.EndDate = &sd, &ed
	}
	st, en := csil.Timestamp(startsAt), csil.Timestamp(endsAt)
	ev.Start, ev.End = &st, &en
	ev.Timezone = optStr(tz)
	es, et, evis := csil.EventStatus(status), csil.EventTransparency(transp), csil.EventVisibility(vis)
	ev.Status, ev.Transparency, ev.Visibility = &es, &et, &evis
	if rrule.Valid {
		ev.Recurrence = &csil.Recurrence{Rrule: rrule.String, Exdates: jsonTimestamps(exdates), Rdates: jsonTimestamps(rdates)}
	} else if rdates.Valid || exdates.Valid {
		// RDATE without RRULE is still a series.
		ev.Recurrence = &csil.Recurrence{Exdates: jsonTimestamps(exdates), Rdates: jsonTimestamps(rdates)}
		if len(ev.Recurrence.Rdates) == 0 {
			ev.Recurrence = nil
		}
	}
	if cats.Valid {
		_ = json.Unmarshal([]byte(cats.String), &ev.Categories)
	}
	ev.Color = optStr(color)
	uv := csil.ActingVia(via)
	ev.UpdatedVia = &uv
	ev.CreatedAt, ev.UpdatedAt = csil.Timestamp(created), csil.Timestamp(updated)
	e.OrganizerID, e.CreatedByID, e.UpdatedByID = strOf(org), strOf(cby), strOf(uby)
	e.SeriesUntil = timeOf(until)
	return e, nil
}

func optStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func strPtrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func jsonTimestamps(ns sql.NullString) []csil.Timestamp {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	var out []csil.Timestamp
	_ = json.Unmarshal([]byte(ns.String), &out)
	return out
}

func jsonOrNull[T any](v []T) any {
	if len(v) == 0 {
		return nil
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// eventArgs returns the column values of e in eventCols order, after id.
func eventArgs(e *Event) ([]any, error) {
	ev := &e.Ev
	start, end, err := Bounds(*ev)
	if err != nil {
		return nil, err
	}
	allDay := ev.AllDay != nil && *ev.AllDay
	var startDate, endDate any
	if allDay {
		startDate, endDate = string(*ev.StartDate), string(*ev.EndDate)
	}
	var master, recID any
	if ev.MasterEventId != nil {
		master = string(*ev.MasterEventId)
	}
	if ev.RecurrenceId != nil {
		recID = string(*ev.RecurrenceId)
	}
	status, transp, vis := "confirmed", "opaque", "default"
	if ev.Status != nil && *ev.Status != "" {
		status = string(*ev.Status)
	}
	if ev.Transparency != nil && *ev.Transparency != "" {
		transp = string(*ev.Transparency)
	}
	if ev.Visibility != nil && *ev.Visibility != "" {
		vis = string(*ev.Visibility)
	}
	var rrule, exdates, rdates any
	if ev.Recurrence != nil {
		rrule = nullStr(ev.Recurrence.Rrule)
		exdates = jsonOrNull(ev.Recurrence.Exdates)
		rdates = jsonOrNull(ev.Recurrence.Rdates)
	}
	via := "self"
	if ev.UpdatedVia != nil && *ev.UpdatedVia != "" {
		via = string(*ev.UpdatedVia)
	}
	return []any{
		string(ev.CalendarId), strPtrVal(ev.Uid), master, recID, ev.Title,
		ev.Description, ev.Location, ev.Url, ev.ConferenceUrl,
		FormatTime(start), FormatTime(end), allDay, startDate, endDate, ev.Timezone,
		status, transp, vis, rrule, exdates, rdates, nullTime(e.SeriesUntil),
		jsonOrNull(ev.Categories), ev.Color, nullStr(e.OrganizerID), nullStr(e.CreatedByID), nullStr(e.UpdatedByID),
		via, ev.Sequence, SearchText(*ev),
	}, nil
}

// insertEvent writes a new row with its attendees and reminders. It sets
// the id and the timestamps on e.
func (s *Store) insertEvent(ctx context.Context, q queryer, e *Event) error {
	if e.Ev.EventId == "" {
		e.Ev.EventId = csil.EventID(NewID())
	}
	now := csil.Timestamp(FormatTime(s.now()))
	e.Ev.CreatedAt, e.Ev.UpdatedAt = now, now
	args, err := eventArgs(e)
	if err != nil {
		return err
	}
	all := append([]any{string(e.Ev.EventId)}, args...)
	all = append(all, string(now), string(now))
	_, err = q.ExecContext(ctx, `INSERT INTO events (id, calendar_id, uid, master_id, recurrence_id, title, description,
		location, url, conference_url, starts_at, ends_at, all_day, start_date, end_date, timezone, status,
		transparency, visibility, rrule, exdates, rdates, series_until, categories, color, organizer_id,
		created_by, updated_by, updated_via, sequence, search_text, created_at, updated_at)
		VALUES (`+placeholders(33)+`)`, all...)
	if isUnique(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	return writeEventChildren(ctx, q, e, s.now())
}

// updateEvent rewrites every column of an existing row and its children.
func (s *Store) updateEvent(ctx context.Context, q queryer, e *Event) error {
	e.Ev.UpdatedAt = csil.Timestamp(FormatTime(s.now()))
	args, err := eventArgs(e)
	if err != nil {
		return err
	}
	args = append(args, string(e.Ev.UpdatedAt), string(e.Ev.EventId))
	res, err := q.ExecContext(ctx, `UPDATE events SET calendar_id = ?, uid = ?, master_id = ?, recurrence_id = ?,
		title = ?, description = ?, location = ?, url = ?, conference_url = ?, starts_at = ?, ends_at = ?,
		all_day = ?, start_date = ?, end_date = ?, timezone = ?, status = ?, transparency = ?, visibility = ?,
		rrule = ?, exdates = ?, rdates = ?, series_until = ?, categories = ?, color = ?, organizer_id = ?,
		created_by = ?, updated_by = ?, updated_via = ?, sequence = ?, search_text = ?, updated_at = ?
		WHERE id = ?`, args...)
	if isUnique(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return writeEventChildren(ctx, q, e, s.now())
}

func writeEventChildren(ctx context.Context, q queryer, e *Event, now time.Time) error {
	id := string(e.Ev.EventId)
	if _, err := q.ExecContext(ctx, `DELETE FROM event_attendees WHERE event_id = ?`, id); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM event_reminders WHERE event_id = ?`, id); err != nil {
		return err
	}
	for i, a := range e.Ev.Attendees {
		var domain, subject any
		if a.Identity != nil && a.Identity.UserId != nil {
			domain, subject = strings.ToLower(a.Identity.Domain), *a.Identity.UserId
		}
		if subject == nil && (a.Email == nil || *a.Email == "") {
			continue
		}
		role, status, rsvp := "required", "needs_action", true
		if a.Role != nil && *a.Role != "" {
			role = string(*a.Role)
		}
		if a.Status != nil && *a.Status != "" {
			status = string(*a.Status)
		}
		if a.Rsvp != nil {
			rsvp = *a.Rsvp
		}
		var respondedBy, respondedAt any
		if i < len(e.RespondedBy) && e.RespondedBy[i] != "" {
			respondedBy, respondedAt = e.RespondedBy[i], FormatTime(now)
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO event_attendees (id, event_id, identity_domain, identity_subject,
			email, name, role, status, rsvp, responded_by, responded_at, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			NewID(), id, domain, subject, a.Email, a.Name, role, status, rsvp, respondedBy, respondedAt, i); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, r := range e.Ev.Reminders {
		method := "display"
		if r.Method != nil && *r.Method != "" {
			method = string(*r.Method)
		}
		key := fmt.Sprintf("%d/%s", r.MinutesBefore, method)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, err := q.ExecContext(ctx, `INSERT INTO event_reminders (event_id, minutes_before, method) VALUES (?, ?, ?)`,
			id, r.MinutesBefore, method); err != nil {
			return err
		}
	}
	return nil
}

// loadEvents runs a query that selects eventCols and fills attendees and
// reminders.
func (s *Store) loadEvents(ctx context.Context, q queryer, query string, args ...any) ([]Event, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.loadChildren(ctx, q, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) loadChildren(ctx context.Context, q queryer, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	idx := map[string]int{}
	ids := make([]any, 0, len(events))
	for i := range events {
		idx[events[i].ID()] = i
		ids = append(ids, events[i].ID())
	}
	// SQLite allows 32766 host parameters; chunk to stay well under it.
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		chunk := ids[start:end]
		rows, err := q.QueryContext(ctx, `SELECT event_id, identity_domain, identity_subject, email, name, role, status,
			rsvp, responded_by FROM event_attendees WHERE event_id IN (`+placeholders(len(chunk))+`)
			ORDER BY event_id, position`, chunk...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var eid, role, status string
			var domain, subject, email, name, respBy sql.NullString
			var rsvp bool
			if err := rows.Scan(&eid, &domain, &subject, &email, &name, &role, &status, &rsvp, &respBy); err != nil {
				rows.Close()
				return err
			}
			a := csil.Attendee{Email: optStr(email), Name: optStr(name)}
			if subject.Valid {
				sub := subject.String
				a.Identity = &csil.IdentityRef{Domain: domain.String, UserId: &sub}
			}
			r, st := csil.AttendeeRole(role), csil.ParticipationStatus(status)
			a.Role, a.Status, a.Rsvp = &r, &st, &rsvp
			e := &events[idx[eid]]
			e.Ev.Attendees = append(e.Ev.Attendees, a)
			e.RespondedBy = append(e.RespondedBy, strOf(respBy))
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = q.QueryContext(ctx, `SELECT event_id, minutes_before, method FROM event_reminders
			WHERE event_id IN (`+placeholders(len(chunk))+`) ORDER BY event_id, minutes_before`, chunk...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var eid, method string
			var minutes int64
			if err := rows.Scan(&eid, &minutes, &method); err != nil {
				rows.Close()
				return err
			}
			m := csil.ReminderMethod(method)
			e := &events[idx[eid]]
			e.Ev.Reminders = append(e.Ev.Reminders, csil.Reminder{MinutesBefore: minutes, Method: &m})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	return nil
}

// CreateEvent stores a new event row.
func (s *Store) CreateEvent(ctx context.Context, e *Event) error {
	return s.InTx(ctx, func(tx *sql.Tx) error { return s.insertEvent(ctx, tx, e) })
}

// UpdateEvent rewrites an event row.
func (s *Store) UpdateEvent(ctx context.Context, e *Event) error {
	return s.InTx(ctx, func(tx *sql.Tx) error { return s.updateEvent(ctx, tx, e) })
}

// EventWrite is one step of a multi-row event change.
type EventWrite struct {
	Insert *Event
	Update *Event
	// DeleteID removes a row (and its overrides by cascade).
	DeleteID string
	// DeleteOverridesFrom removes the overrides of a master whose
	// recurrence id is at or after From.
	DeleteOverridesOf string
	From              time.Time
}

// ApplyEventWrites runs several event writes in one transaction.
func (s *Store) ApplyEventWrites(ctx context.Context, writes []EventWrite) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		for _, w := range writes {
			switch {
			case w.Insert != nil:
				if err := s.insertEvent(ctx, tx, w.Insert); err != nil {
					return err
				}
			case w.Update != nil:
				if err := s.updateEvent(ctx, tx, w.Update); err != nil {
					return err
				}
			case w.DeleteID != "":
				if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, w.DeleteID); err != nil {
					return err
				}
			case w.DeleteOverridesOf != "":
				if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE master_id = ? AND recurrence_id >= ?`,
					w.DeleteOverridesOf, FormatTime(w.From)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// GetEvent reads one event row.
func (s *Store) GetEvent(ctx context.Context, id string) (Event, error) {
	evs, err := s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE id = ?`, id)
	if err != nil {
		return Event{}, err
	}
	if len(evs) == 0 {
		return Event{}, ErrNotFound
	}
	return evs[0], nil
}

// DeleteEvent removes a row; overrides go with their master.
func (s *Store) DeleteEvent(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOverrides returns the overrides of a master.
func (s *Store) ListOverrides(ctx context.Context, masterID string) ([]Event, error) {
	return s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE master_id = ? ORDER BY recurrence_id`, masterID)
}

// FindMasterByUID finds the master or single event with a UID.
func (s *Store) FindMasterByUID(ctx context.Context, calendarID, uid string) (Event, error) {
	evs, err := s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE calendar_id = ? AND uid = ? AND master_id IS NULL`,
		calendarID, uid)
	if err != nil {
		return Event{}, err
	}
	if len(evs) == 0 {
		return Event{}, ErrNotFound
	}
	return evs[0], nil
}

// FindOverride finds the override of one instance.
func (s *Store) FindOverride(ctx context.Context, masterID string, recurrenceID time.Time) (Event, error) {
	evs, err := s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE master_id = ? AND recurrence_id = ?`,
		masterID, FormatTime(recurrenceID))
	if err != nil {
		return Event{}, err
	}
	if len(evs) == 0 {
		return Event{}, ErrNotFound
	}
	return evs[0], nil
}

// ListCalendarEvents returns every row of a calendar.
func (s *Store) ListCalendarEvents(ctx context.Context, calendarID string) ([]Event, error) {
	return s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE calendar_id = ?
		ORDER BY master_id IS NOT NULL, starts_at, id`, calendarID)
}

// ListEventsInRange returns the masters and single events of the calendars
// that can have an instance in [start, end), plus every override of those
// masters (an override can move an instance into the range).
func (s *Store) ListEventsInRange(ctx context.Context, calendarIDs []string, start, end time.Time) ([]Event, error) {
	if len(calendarIDs) == 0 {
		return nil, nil
	}
	args := anySlice(calendarIDs)
	st, en := FormatTime(start), FormatTime(end)
	// Masters and singles. A single event overlaps when it starts before the
	// end and ends after the start. A series is a candidate until its
	// series_until (NULL = open).
	q := `SELECT ` + eventCols + ` FROM events WHERE calendar_id IN (` + placeholders(len(args)) + `)
		AND master_id IS NULL AND starts_at < ?
		AND ((rrule IS NULL AND rdates IS NULL AND ends_at > ?)
		  OR ((rrule IS NOT NULL OR rdates IS NOT NULL) AND (series_until IS NULL OR series_until > ?)))
		ORDER BY starts_at, id`
	masters, err := s.loadEvents(ctx, s.DB, q, append(append([]any{}, args...), en, st, st)...)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, m := range masters {
		have[m.ID()] = true
	}
	// Masters of overrides that moved into the range.
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT master_id FROM events WHERE calendar_id IN (`+placeholders(len(args))+`)
		AND master_id IS NOT NULL AND starts_at < ? AND ends_at > ?`, append(append([]any{}, args...), en, st)...)
	if err != nil {
		return nil, err
	}
	var extra []any
	for rows.Next() {
		var mid string
		if err := rows.Scan(&mid); err != nil {
			rows.Close()
			return nil, err
		}
		if !have[mid] {
			extra = append(extra, mid)
			have[mid] = true
		}
	}
	rows.Close()
	if len(extra) > 0 {
		more, err := s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE id IN (`+placeholders(len(extra))+`)`, extra...)
		if err != nil {
			return nil, err
		}
		masters = append(masters, more...)
	}
	var seriesIDs []any
	for _, m := range masters {
		if m.Ev.Recurrence != nil {
			seriesIDs = append(seriesIDs, m.ID())
		}
	}
	out := masters
	for startIdx := 0; startIdx < len(seriesIDs); startIdx += 500 {
		chunk := seriesIDs[startIdx:min(startIdx+500, len(seriesIDs))]
		ovs, err := s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE master_id IN (`+placeholders(len(chunk))+`)
			ORDER BY recurrence_id`, chunk...)
		if err != nil {
			return nil, err
		}
		out = append(out, ovs...)
	}
	return out, nil
}

// InvitationRows returns the masters and single events (plus overrides of
// those masters) that name the identity as an attendee, on calendars not
// owned by excludeOwnerID.
func (s *Store) InvitationRows(ctx context.Context, domain, subject, excludeOwnerID string) ([]Event, error) {
	masters, err := s.loadEvents(ctx, s.DB, `SELECT `+prefixCols("e", eventCols)+` FROM events e
		JOIN calendars c ON c.id = e.calendar_id
		WHERE e.master_id IS NULL AND c.owner_id <> ?
		AND EXISTS (SELECT 1 FROM event_attendees a WHERE a.event_id = e.id AND a.identity_domain = ? AND a.identity_subject = ?)
		ORDER BY e.starts_at, e.id`, excludeOwnerID, strings.ToLower(domain), subject)
	if err != nil {
		return nil, err
	}
	var seriesIDs []any
	for _, m := range masters {
		if m.Ev.Recurrence != nil {
			seriesIDs = append(seriesIDs, m.ID())
		}
	}
	if len(seriesIDs) == 0 {
		return masters, nil
	}
	ovs, err := s.loadEvents(ctx, s.DB, `SELECT `+eventCols+` FROM events WHERE master_id IN (`+placeholders(len(seriesIDs))+`)`, seriesIDs...)
	if err != nil {
		return nil, err
	}
	return append(masters, ovs...), nil
}

// SearchEvents finds masters and single events whose folded text contains
// the folded query, on the given calendars or naming the identity as an
// attendee.
func (s *Store) SearchEvents(ctx context.Context, calendarIDs []string, domain, subject, query string, limit int) ([]Event, error) {
	pattern := "%" + likeEscape(strings.ToLower(query)) + "%"
	args := []any{pattern}
	calClause := "0"
	if len(calendarIDs) > 0 {
		calClause = "e.calendar_id IN (" + placeholders(len(calendarIDs)) + ")"
		args = append(args, anySlice(calendarIDs)...)
	}
	args = append(args, strings.ToLower(domain), subject, limit)
	return s.loadEvents(ctx, s.DB, `SELECT `+prefixCols("e", eventCols)+` FROM events e
		WHERE e.master_id IS NULL AND e.search_text LIKE ? ESCAPE '\'
		AND (`+calClause+` OR EXISTS (SELECT 1 FROM event_attendees a WHERE a.event_id = e.id
			AND a.identity_domain = ? AND a.identity_subject = ?))
		ORDER BY e.starts_at DESC, e.id LIMIT ?`, args...)
}

func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// MoveEvent moves a master (or single event) and its overrides to another
// calendar.
func (s *Store) MoveEvent(ctx context.Context, id, calendarID string) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		now := FormatTime(s.now())
		res, err := tx.ExecContext(ctx, `UPDATE events SET calendar_id = ?, updated_at = ?, sequence = sequence + 1 WHERE id = ?`,
			calendarID, now, id)
		if isUnique(err) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE events SET calendar_id = ?, updated_at = ? WHERE master_id = ?`, calendarID, now, id)
		return err
	})
}

// ReplaceCalendarEvents deletes every event of a calendar and inserts the
// given ones, in one transaction. Masters and singles go first, then
// overrides, which find their master by UID. It records the sync time and
// clears the sync error.
func (s *Store) ReplaceCalendarEvents(ctx context.Context, calendarID string, events []Event) (skipped int, err error) {
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE calendar_id = ?`, calendarID); err != nil {
			return err
		}
		masters := map[string]string{}
		for i := range events {
			e := &events[i]
			if e.Ev.RecurrenceId != nil {
				continue
			}
			e.Ev.CalendarId = csil.CalendarID(calendarID)
			e.Ev.EventId = ""
			uid := strPtrVal(e.Ev.Uid)
			if _, dup := masters[uid]; dup {
				skipped++
				continue
			}
			if err := s.insertEvent(ctx, tx, e); err != nil {
				if errors.Is(err, ErrConflict) {
					skipped++
					continue
				}
				return err
			}
			masters[uid] = e.ID()
		}
		for i := range events {
			e := &events[i]
			if e.Ev.RecurrenceId == nil {
				continue
			}
			mid, ok := masters[strPtrVal(e.Ev.Uid)]
			if !ok {
				skipped++
				continue
			}
			m := csil.EventID(mid)
			e.Ev.MasterEventId = &m
			e.Ev.CalendarId = csil.CalendarID(calendarID)
			e.Ev.EventId = ""
			if err := s.insertEvent(ctx, tx, e); err != nil {
				if errors.Is(err, ErrConflict) {
					skipped++
					continue
				}
				return err
			}
		}
		now := FormatTime(s.now())
		_, err := tx.ExecContext(ctx, `UPDATE calendars SET last_synced_at = ?, sync_error = NULL, updated_at = ? WHERE id = ?`,
			now, now, calendarID)
		return err
	})
	return skipped, err
}

// SetAttendeeStatus records an answer of the attendee with the identity on
// one event row. respondedBy is the delegate's user id, or "".
func (s *Store) SetAttendeeStatus(ctx context.Context, eventID, domain, subject, status, respondedBy string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE event_attendees SET status = ?, responded_by = ?, responded_at = ?
		WHERE event_id = ? AND identity_domain = ? AND identity_subject = ?`,
		status, nullStr(respondedBy), FormatTime(s.now()), eventID, strings.ToLower(domain), subject)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
