import type { Occurrence, ParticipationStatus } from "@catalystcal/client";
import { eventClient, icsClient } from "~/data/clients";
import { bumpEvents, calendarById, canAddEvents, canEditEvent, defaultWritableCalendar } from "~/stores/calendars";
import { effectivePerson, hasScope, isActing } from "~/stores/auth";
import { displayZone } from "~/stores/prefs";
import { closeDetail, openEditor } from "~/stores/editor";
import { askScope, confirmAction } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import { STATUS_LABEL, myAttendee, occColor, occTitle } from "~/lib/eventVisual";
import { allDayLength, allDayStart } from "~/lib/occurrence";
import { addDays } from "~/lib/dates";
import { civilDate, formatDate, formatTimeRange, parseTimestamp, zoneAbbrev } from "~/lib/tz";
import { describeRRule } from "~/lib/rrule";
import { buildDelete, duplicateForm, formFromOccurrence, isSeriesOccurrence } from "~/lib/eventForm";
import { identityLabel, personName, samePerson } from "~/lib/people";
import { downloadText } from "~/lib/download";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, type Child } from "~/ui/dom";
import { icon, type IconName } from "~/ui/icons";
import { openModal, type CheckControl, type DialogWindow, type PushButton } from "~/ui/controls";

const RSVP: { status: ParticipationStatus; label: string }[] = [
  { status: "accepted", label: "Accept" },
  { status: "tentative", label: "Maybe" },
  { status: "declined", label: "Decline" },
];

export function whenText(o: Occurrence, tz: string): string {
  if (o.event.allDay) {
    const s = allDayStart(o, tz);
    const len = allDayLength(o);
    const a = formatDate(s, { weekday: "long", month: "long", day: "numeric" });
    return len > 1 ? `${a} – ${formatDate(addDays(s, len - 1), { weekday: "long", month: "long", day: "numeric" })}` : `${a}, all day`;
  }
  const s = parseTimestamp(o.start);
  const e = parseTimestamp(o.end);
  const sd = civilDate(s, tz);
  const ed = civilDate(e - 1, tz);
  const day = formatDate(sd, { weekday: "long", month: "long", day: "numeric" });
  if (sd === ed) return `${day}, ${formatTimeRange(s, e, tz)}`;
  return `${day}, ${formatTimeRange(s, s, tz).split(" – ")[0]} – ${formatDate(ed, { weekday: "short", month: "short", day: "numeric" })}, ${formatTimeRange(e, e, tz).split(" – ")[0]}`;
}

export function reminderText(min: number): string {
  if (min === 0) return "At the start";
  if (min % 1440 === 0) return `${min / 1440} day${min === 1440 ? "" : "s"} before`;
  if (min % 60 === 0) return `${min / 60} hour${min === 60 ? "" : "s"} before`;
  return `${min} minutes before`;
}

const line = (name: IconName, ...children: Child[]) => h("div", { class: "detail-line" }, icon(name), ...children);

const statusPill = (status: ParticipationStatus | undefined) =>
  h(
    "span",
    { class: `pill${status === "accepted" ? " pill-ok" : status === "declined" ? " pill-danger" : status === "tentative" ? " pill-warn" : ""}` },
    STATUS_LABEL[status ?? "needs_action"],
  );

/**
 * The detail dialog of one occurrence. Set `occ` before the element
 * connects; it opens a modal at the first connect, and calls closeDetail()
 * when the modal closes.
 */
export class EventDetail extends CalElement {
  declare occ: Occurrence;
  #dialog: DialogWindow | null = null;
  #opened = false;
  #content!: HTMLElement;
  #actions: Child[] = [];
  #status: ParticipationStatus | undefined;
  #rsvp: { status: ParticipationStatus; button: PushButton }[] = [];
  #onlyThis: CheckControl | null = null;

  protected build(): void {
    const o = this.occ;
    const ev = o.event;
    const tz = displayZone();
    const cal = calendarById(ev.calendarId);
    const me = effectivePerson();
    const mine = myAttendee(o, me);
    this.#status = mine?.status;
    const canWrite = !o.busyOnly && canEditEvent(o.access, ev.createdBy?.userId, me?.userId) && !(o.invited && !cal);
    const canRespond = hasScope("events.respond");
    const showRsvp = !o.busyOnly && (o.invited || !!mine);
    const isOrganizer = samePerson(ev.organizer, me);
    const eventTz = ev.timezone;
    const otherZone =
      !ev.allDay && !!eventTz && eventTz !== tz && zoneAbbrev(parseTimestamp(o.start), eventTz) !== zoneAbbrev(parseTimestamp(o.start), tz);

    const parts: Child[] = [];
    parts.push(
      line(
        "clock",
        h(
          "div",
          null,
          h("p", null, whenText(o, tz)),
          otherZone ? h("p", { class: "small muted" }, `${whenText(o, eventTz!)} (${eventTz!.replace(/_/g, " ")})`) : null,
          ev.recurrence
            ? h(
                "p",
                { class: "small muted row-inline" },
                icon("repeat", 13),
                describeRRule(
                  ev.recurrence.rrule,
                  ev.allDay ? (ev.startDate ?? civilDate(parseTimestamp(o.start), tz)) : civilDate(parseTimestamp(ev.start ?? o.start), eventTz || tz),
                  eventTz || tz,
                ),
              )
            : null,
        ),
      ),
    );
    parts.push(
      h(
        "div",
        { class: "detail-line" },
        h("span", { class: "swatch detail-swatch", style: { "--sw": occColor(o) } }),
        h(
          "div",
          null,
          h("p", null, cal?.name ?? (o.invited ? "Invitation" : "Calendar")),
          cal && !samePerson(cal.owner, me) ? h("p", { class: "small muted" }, `${personName(cal.owner)}’s calendar`) : null,
        ),
      ),
    );

    if (o.busyOnly) parts.push(h("p", { class: "notice" }, "You can see only that this time is busy."));
    else {
      if (ev.location) parts.push(line("pin", h("p", null, ev.location)));
      if (ev.conferenceUrl)
        parts.push(
          line(
            "video",
            h("a", { href: ev.conferenceUrl, target: "_blank", rel: "noopener noreferrer", class: "btn btn-sm" }, "Join call"),
            h("span", { class: "small muted truncate grow" }, ev.conferenceUrl),
          ),
        );
      if (ev.url) parts.push(line("link", h("a", { href: ev.url, target: "_blank", rel: "noopener noreferrer", class: "truncate" }, ev.url)));

      if (showRsvp) {
        this.#rsvp = RSVP.map((r) => ({
          status: r.status,
          button: h("push-button", { toggle: true, disabled: !canRespond, onClick: () => void this.#respond(r.status) }, r.label) as PushButton,
        }));
        this.#onlyThis = isSeriesOccurrence(o) ? (h("check-box", { class: "small" }, "Answer for this occurrence only") as CheckControl) : null;
        parts.push(
          h(
            "section",
            { class: "rsvp", "aria-labelledby": "rsvp-h" },
            h("h3", { id: "rsvp-h" }, "Going?", isActing() ? h("span", { class: "small muted" }, ` (answering as ${personName(me)})`) : null),
            h("div", { class: "segmented rsvp-buttons", role: "group", "aria-labelledby": "rsvp-h" }, this.#rsvp.map((r) => r.button)),
            this.#onlyThis,
            canRespond ? null : h("p", { class: "small muted" }, "Your delegation does not let you answer invitations for this person."),
          ),
        );
        this.#syncRsvp();
      }

      const attendees = (ev.attendees ?? []).filter((a) => !(ev.organizer && a.person && samePerson(a.person, ev.organizer)));
      if ((ev.attendees ?? []).length > 0 || ev.organizer) {
        parts.push(
          h(
            "section",
            { "aria-labelledby": "ppl-h" },
            h("h3", { id: "ppl-h", class: "detail-h" }, "People"),
            h(
              "ul",
              { class: "list people-list" },
              ev.organizer ? h("li", null, h("span", { class: "grow truncate" }, personName(ev.organizer)), h("span", { class: "pill pill-accent" }, "Organizer")) : null,
              attendees.map((a) =>
                h(
                  "li",
                  null,
                  h(
                    "span",
                    { class: "grow" },
                    h("span", { class: "truncate", style: { display: "block" } }, a.name || (a.identity ? identityLabel(a.identity, a.person) : (a.email ?? ""))),
                    a.respondedBy ? h("span", { class: "small muted" }, `answered by ${personName(a.respondedBy)}`) : null,
                    a.role === "optional" ? h("span", { class: "small muted" }, " Optional") : null,
                  ),
                  statusPill(a.status),
                ),
              ),
            ),
          ),
        );
      }

      if (ev.description)
        parts.push(h("section", { "aria-labelledby": "desc-h" }, h("h3", { id: "desc-h", class: "detail-h" }, "Description"), h("p", { class: "detail-desc" }, ev.description)));
      if ((ev.reminders ?? []).length) parts.push(line("bell", h("p", { class: "small" }, (ev.reminders ?? []).map((r) => reminderText(r.minutesBefore)).join(", "))));
      if ((ev.categories ?? []).length) parts.push(h("div", { class: "hrow" }, (ev.categories ?? []).map((c) => h("span", { class: "pill" }, c))));
      if (ev.updatedVia === "delegation" || ev.updatedVia === "act_as_grant")
        parts.push(
          h(
            "p",
            { class: "small muted detail-audit" },
            `Last changed for ${personName(ev.updatedBy)} ${ev.updatedVia === "delegation" ? "by a delegate" : "by an application they allowed"}. See Activity for who did it.`,
          ),
        );
      if (!isOrganizer && ev.createdBy && !samePerson(ev.createdBy, ev.organizer)) parts.push(h("p", { class: "small muted" }, `Created by ${personName(ev.createdBy)}`));
    }
    this.#content = h("div", { class: "detail", style: { "--ev": occColor(o) } }, parts);

    const action = (name: IconName, label: string, run: () => void, variant = "default") =>
      h("push-button", { variant, onClick: run }, icon(name), h("span", null, label));
    if (canWrite) this.#actions.push(action("edit", "Edit", () => this.#edit(), "primary"));
    if (!o.busyOnly) {
      this.#actions.push(action("copy", "Duplicate", () => this.#duplicate()));
      this.#actions.push(action("download", "Export .ics", () => void this.#exportIcs()));
    }
    if (canWrite) this.#actions.push(action("trash", "Delete", () => void this.#remove(), "danger"));
  }

  protected connected(): void {
    if (this.#opened) return;
    this.#opened = true;
    const { dialog, result } = openModal({ label: occTitle(this.occ), className: "detail-dialog", children: [this.#content], buttons: this.#actions });
    this.#dialog = dialog;
    void result.then(() => {
      this.#dialog = null;
      closeDetail();
    });
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#dialog?.close("cancel");
  }

  #syncRsvp(): void {
    for (const r of this.#rsvp) if (r.button.pressed !== (this.#status === r.status)) r.button.pressed = this.#status === r.status;
  }

  async #respond(s: ParticipationStatus): Promise<void> {
    // The button toggles itself on a click; the answer decides.
    this.#syncRsvp();
    for (const r of this.#rsvp) r.button.disabled = true;
    const o = this.occ;
    try {
      await eventClient.respond({ eventId: o.event.eventId, status: s, recurrenceId: this.#onlyThis?.checked ? (o.recurrenceId ?? o.start) : undefined });
      this.#status = s;
      bumpEvents();
      toast(`Answered: ${STATUS_LABEL[s]}${isActing() ? `, as ${personName(effectivePerson())}` : ""}.`);
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      for (const r of this.#rsvp) r.button.disabled = false;
      this.#syncRsvp();
    }
  }

  async #exportIcs(): Promise<void> {
    const ev = this.occ.event;
    try {
      const doc = await icsClient.exportEvent({ eventId: ev.eventId, method: "request" });
      downloadText(doc.filename || `${ev.title || "event"}.ics`, doc.ics);
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }

  async #remove(): Promise<void> {
    const o = this.occ;
    let scope: "all" | "this" | "this_and_future" | null = "all";
    if (isSeriesOccurrence(o)) scope = await askScope("delete");
    else {
      const ok = await confirmAction({ title: "Delete event", message: `Delete “${o.event.title}”? Attendees get a cancellation.`, confirmLabel: "Delete", danger: true });
      scope = ok ? "all" : null;
    }
    if (!scope) return;
    try {
      await eventClient.deleteEvent(buildDelete(o, scope));
      bumpEvents();
      toast("Event deleted.");
      this.#dialog?.close("deleted");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }

  #edit(): void {
    openEditor({ mode: "edit", occ: this.occ, form: formFromOccurrence(this.occ, displayZone()) });
  }

  #duplicate(): void {
    const o = this.occ;
    const f = duplicateForm(o, displayZone());
    const cal = calendarById(o.event.calendarId);
    if (!cal || !canAddEvents(cal.access)) {
      const d = defaultWritableCalendar(effectivePerson()?.userId);
      if (!d) return toast("You have no calendar you can add events to.", "error");
      f.calendarId = d.calendarId;
    }
    openEditor({ mode: "create", form: f });
  }
}
define("cal-event-detail", EventDetail);
