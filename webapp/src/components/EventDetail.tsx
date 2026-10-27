import { For, Show, createSignal } from "solid-js";
import type { Occurrence, ParticipationStatus } from "@catalystcal/client";
import { Modal } from "~/components/Modal";
import { Icon } from "~/components/Icons";
import { eventClient, icsClient } from "~/data/clients";
import { accessAtLeast, bumpEvents, calendarById, defaultWritableCalendar } from "~/stores/calendars";
import { effectivePerson, hasScope, isActing } from "~/stores/auth";
import { displayZone } from "~/stores/prefs";
import { openEditor } from "~/stores/editor";
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

export function EventDetail(props: { occ: Occurrence; onClose: () => void }) {
  const o = () => props.occ;
  const ev = () => props.occ.event;
  const tz = () => displayZone();
  const cal = () => calendarById(ev().calendarId);
  const me = () => myAttendee(o(), effectivePerson());
  const [status, setStatus] = createSignal<ParticipationStatus | undefined>(me()?.status);
  const [onlyThis, setOnlyThis] = createSignal(false);
  const [busy, setBusy] = createSignal(false);

  const canWrite = () => !o().busyOnly && accessAtLeast(o().access, "write") && !(o().invited && !cal());
  const canRespond = () => hasScope("events.respond");
  const showRsvp = () => !o().busyOnly && (o().invited || !!me());
  const isOrganizer = () => samePerson(ev().organizer, effectivePerson());

  const respond = async (s: ParticipationStatus) => {
    setBusy(true);
    try {
      await eventClient.respond({ eventId: ev().eventId, status: s, recurrenceId: onlyThis() ? (o().recurrenceId ?? o().start) : undefined });
      setStatus(s);
      bumpEvents();
      toast(`Answered: ${STATUS_LABEL[s]}${isActing() ? `, as ${personName(effectivePerson())}` : ""}.`);
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      setBusy(false);
    }
  };

  const exportIcs = async () => {
    try {
      const doc = await icsClient.exportEvent({ eventId: ev().eventId, method: "request" });
      downloadText(doc.filename || `${ev().title || "event"}.ics`, doc.ics);
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  };

  const remove = async () => {
    let scope: "all" | "this" | "this_and_future" | null = "all";
    if (isSeriesOccurrence(o())) {
      scope = await askScope("delete");
    } else {
      const ok = await confirmAction({ title: "Delete event", message: `Delete “${ev().title}”? Attendees get a cancellation.`, confirmLabel: "Delete", danger: true });
      scope = ok ? "all" : null;
    }
    if (!scope) return;
    try {
      await eventClient.deleteEvent(buildDelete(o(), scope));
      bumpEvents();
      toast("Event deleted.");
      props.onClose();
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  };

  const edit = () => openEditor({ mode: "edit", occ: o(), form: formFromOccurrence(o(), tz()) });
  const duplicate = () => {
    const f = duplicateForm(o(), tz());
    if (!cal() || !accessAtLeast(cal()!.access, "write")) {
      const d = defaultWritableCalendar(effectivePerson()?.userId);
      if (!d) return toast("You have no calendar you can add events to.", "error");
      f.calendarId = d.calendarId;
    }
    openEditor({ mode: "create", form: f });
  };

  const eventTz = () => ev().timezone;
  const otherZone = () => !ev().allDay && eventTz() && eventTz() !== tz() && zoneAbbrev(parseTimestamp(o().start), eventTz()!) !== zoneAbbrev(parseTimestamp(o().start), tz());

  return (
    <Modal
      title={occTitle(o())}
      variant="sheet"
      onClose={props.onClose}
      footer={
        <div class="row detail-actions">
          <Show when={canWrite()}>
            <button type="button" class="btn btn-primary" onClick={edit}>
              <Icon name="edit" /> Edit
            </button>
          </Show>
          <Show when={!o().busyOnly}>
            <button type="button" class="btn" onClick={duplicate}>
              <Icon name="copy" /> Duplicate
            </button>
            <button type="button" class="btn" onClick={exportIcs}>
              <Icon name="download" /> Export .ics
            </button>
          </Show>
          <Show when={canWrite()}>
            <button type="button" class="btn btn-danger" onClick={remove}>
              <Icon name="trash" /> Delete
            </button>
          </Show>
        </div>
      }
    >
      <div class="detail" style={{ "--ev": occColor(o()) }}>
        <div class="detail-line">
          <Icon name="clock" />
          <div>
            <p>{whenText(o(), tz())}</p>
            <Show when={otherZone()}>
              <p class="small muted">
                {whenText(o(), eventTz()!)} ({eventTz()!.replace(/_/g, " ")})
              </p>
            </Show>
            <Show when={ev().recurrence}>
              <p class="small muted row-inline">
                <Icon name="repeat" size={13} />
                {describeRRule(ev().recurrence!.rrule, ev().allDay ? (ev().startDate ?? civilDate(parseTimestamp(o().start), tz())) : civilDate(parseTimestamp(ev().start ?? o().start), eventTz() || tz()), eventTz() || tz())}
              </p>
            </Show>
          </div>
        </div>

        <div class="detail-line">
          <span class="swatch detail-swatch" style={{ "--sw": occColor(o()) }} />
          <div>
            <p>{cal()?.name ?? (o().invited ? "Invitation" : "Calendar")}</p>
            <Show when={cal() && !samePerson(cal()!.owner, effectivePerson())}>
              <p class="small muted">{personName(cal()!.owner)}’s calendar</p>
            </Show>
          </div>
        </div>

        <Show when={o().busyOnly}>
          <p class="notice">You can see only that this time is busy.</p>
        </Show>

        <Show when={!o().busyOnly}>
          <Show when={ev().location}>
            <div class="detail-line">
              <Icon name="pin" />
              <p>{ev().location}</p>
            </div>
          </Show>
          <Show when={ev().conferenceUrl}>
            <div class="detail-line">
              <Icon name="video" />
              <a href={ev().conferenceUrl} target="_blank" rel="noopener noreferrer" class="btn btn-sm">
                Join call
              </a>
              <span class="small muted truncate grow">{ev().conferenceUrl}</span>
            </div>
          </Show>
          <Show when={ev().url}>
            <div class="detail-line">
              <Icon name="link" />
              <a href={ev().url} target="_blank" rel="noopener noreferrer" class="truncate">
                {ev().url}
              </a>
            </div>
          </Show>

          <Show when={showRsvp()}>
            <section class="rsvp" aria-labelledby="rsvp-h">
              <h3 id="rsvp-h">
                Going?
                <Show when={isActing()}>
                  <span class="small muted"> (answering as {personName(effectivePerson())})</span>
                </Show>
              </h3>
              <div class="segmented rsvp-buttons" role="group" aria-labelledby="rsvp-h">
                <For each={RSVP}>
                  {(r) => (
                    <button type="button" aria-pressed={status() === r.status} disabled={busy() || !canRespond()} onClick={() => respond(r.status)}>
                      {r.label}
                    </button>
                  )}
                </For>
              </div>
              <Show when={isSeriesOccurrence(o())}>
                <label class="check small">
                  <input type="checkbox" checked={onlyThis()} onChange={(e) => setOnlyThis(e.currentTarget.checked)} />
                  Answer for this occurrence only
                </label>
              </Show>
              <Show when={!canRespond()}>
                <p class="small muted">Your delegation does not let you answer invitations for this person.</p>
              </Show>
            </section>
          </Show>

          <Show when={(ev().attendees ?? []).length > 0 || ev().organizer}>
            <section aria-labelledby="ppl-h">
              <h3 id="ppl-h" class="detail-h">
                People
              </h3>
              <ul class="list people-list">
                <Show when={ev().organizer}>
                  <li>
                    <span class="grow truncate">{personName(ev().organizer)}</span>
                    <span class="pill pill-accent">Organizer</span>
                  </li>
                </Show>
                <For each={(ev().attendees ?? []).filter((a) => !(ev().organizer && a.person && samePerson(a.person, ev().organizer)))}>
                  {(a) => (
                    <li>
                      <span class="grow">
                        <span class="truncate" style={{ display: "block" }}>
                          {a.name || (a.identity ? identityLabel(a.identity, a.person) : a.email)}
                        </span>
                        <Show when={a.respondedBy}>
                          <span class="small muted">answered by {personName(a.respondedBy)}</span>
                        </Show>
                        <Show when={a.role === "optional"}>
                          <span class="small muted"> Optional</span>
                        </Show>
                      </span>
                      <span
                        class="pill"
                        classList={{ "pill-ok": a.status === "accepted", "pill-danger": a.status === "declined", "pill-warn": a.status === "tentative" }}
                      >
                        {STATUS_LABEL[a.status ?? "needs_action"]}
                      </span>
                    </li>
                  )}
                </For>
              </ul>
            </section>
          </Show>

          <Show when={ev().description}>
            <section aria-labelledby="desc-h">
              <h3 id="desc-h" class="detail-h">
                Description
              </h3>
              <p class="detail-desc">{ev().description}</p>
            </section>
          </Show>

          <Show when={(ev().reminders ?? []).length}>
            <div class="detail-line">
              <Icon name="bell" />
              <p class="small">{(ev().reminders ?? []).map((r) => reminderText(r.minutesBefore)).join(", ")}</p>
            </div>
          </Show>

          <Show when={(ev().categories ?? []).length}>
            <div class="row">
              <For each={ev().categories}>{(c) => <span class="pill">{c}</span>}</For>
            </div>
          </Show>

          <Show when={ev().updatedVia === "delegation" || ev().updatedVia === "act_as_grant"}>
            <p class="small muted detail-audit">
              Last changed for {personName(ev().updatedBy)} {ev().updatedVia === "delegation" ? "by a delegate" : "by an application they allowed"}. See Activity for who did it.
            </p>
          </Show>
          <Show when={!isOrganizer() && ev().createdBy && !samePerson(ev().createdBy, ev().organizer)}>
            <p class="small muted">Created by {personName(ev().createdBy)}</p>
          </Show>
        </Show>
      </div>
    </Modal>
  );
}

export function reminderText(min: number): string {
  if (min === 0) return "At the start";
  if (min % 1440 === 0) return `${min / 1440} day${min === 1440 ? "" : "s"} before`;
  if (min % 60 === 0) return `${min / 60} hour${min === 60 ? "" : "s"} before`;
  return `${min} minutes before`;
}
