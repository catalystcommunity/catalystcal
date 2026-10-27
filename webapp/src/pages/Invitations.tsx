import { For, Show, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { Event, ParticipationStatus } from "@catalystcal/client";
import { eventClient } from "~/data/clients";
import { effectivePerson, hasScope, isActing, useSession } from "~/stores/auth";
import { bumpEvents, useEventsVersion } from "~/stores/calendars";
import { displayZone } from "~/stores/prefs";
import { goToDate } from "~/stores/nav";
import { openDetail } from "~/stores/editor";
import { toast } from "~/stores/toast";
import { STATUS_LABEL, myAttendee } from "~/lib/eventVisual";
import { eventDate, occurrenceOf } from "~/components/SearchBox";
import { formatDate, formatTimeRange, parseTimestamp } from "~/lib/tz";
import { personName } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";

const ACTIONS: { status: ParticipationStatus; label: string }[] = [
  { status: "accepted", label: "Accept" },
  { status: "tentative", label: "Maybe" },
  { status: "declined", label: "Decline" },
];

export function InvitationsPage() {
  const session = useSession();
  const version = useEventsVersion();
  const navigate = useNavigate();
  const [answered, setAnswered] = createSignal<Record<string, ParticipationStatus>>({});
  const [invites, { refetch }] = createResource(
    () => (session() ? { v: version(), who: effectivePerson()?.userId } : false),
    () => eventClient.listInvitations({}),
  );
  const list = (): Event[] => (invites.error ? [] : (invites.latest?.events ?? []));
  const tz = () => displayZone();
  const statusOf = (e: Event): ParticipationStatus => answered()[e.eventId] ?? myAttendee({ event: e })?.status ?? "needs_action";
  const pending = () => list().filter((e) => statusOf(e) === "needs_action");
  const done = () => list().filter((e) => statusOf(e) !== "needs_action");

  const respond = async (e: Event, status: ParticipationStatus) => {
    try {
      await eventClient.respond({ eventId: e.eventId, status });
      setAnswered({ ...answered(), [e.eventId]: status });
      bumpEvents();
      toast(`${STATUS_LABEL[status]}: ${e.title}`);
    } catch (err) {
      toast(errorMessage(err), "error");
      void refetch();
    }
  };

  const when = (e: Event) => {
    const d = eventDate(e, tz());
    if (!d) return "";
    const day = formatDate(d, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
    if (e.allDay || !e.start || !e.end) return `${day}, all day`;
    return `${day}, ${formatTimeRange(parseTimestamp(e.start), parseTimestamp(e.end), tz())}`;
  };

  const Row = (p: { e: Event }) => (
    <li class="invite">
      <div class="grow">
        <button
          type="button"
          class="link-btn invite-title"
          onClick={() => {
            const d = eventDate(p.e, tz());
            if (d) goToDate(d, "day");
            navigate("/");
            openDetail(occurrenceOf(p.e, true));
          }}
        >
          {p.e.title}
        </button>
        <p class="small muted">
          {when(p.e)}
          <Show when={p.e.organizer}> · from {personName(p.e.organizer)}</Show>
          <Show when={p.e.recurrence}> · repeats</Show>
        </p>
        <Show when={p.e.location}>
          <p class="small muted">{p.e.location}</p>
        </Show>
      </div>
      <div class="segmented" role="group" aria-label={`Answer ${p.e.title}`}>
        <For each={ACTIONS}>
          {(a) => (
            <button type="button" aria-pressed={statusOf(p.e) === a.status} disabled={!hasScope("events.respond")} onClick={() => respond(p.e, a.status)}>
              {a.label}
            </button>
          )}
        </For>
      </div>
    </li>
  );

  return (
    <div class="page">
      <header class="page-head-text">
        <h1>Invitations</h1>
        <p class="lede">
          <Show when={isActing()} fallback="Events other people invited you to.">
            Invitations sent to {personName(effectivePerson())}. Your answers are sent in their name.
          </Show>
        </p>
      </header>
      <Show when={invites.error}>
        <p class="alert" role="alert">
          Invitations did not load: {errorMessage(invites.error)}
        </p>
      </Show>
      <Show when={!hasScope("events.respond")}>
        <p class="notice">Your delegation does not let you answer invitations for this person.</p>
      </Show>
      <section class="card" aria-labelledby="inv-new">
        <h2 id="inv-new">Waiting for an answer</h2>
        <ul class="list">
          <For each={pending()}>{(e) => <Row e={e} />}</For>
        </ul>
        <Show when={!invites.loading && pending().length === 0}>
          <p class="empty">You have answered every invitation.</p>
        </Show>
      </section>
      <Show when={done().length > 0}>
        <section class="card" aria-labelledby="inv-done">
          <h2 id="inv-done">Answered</h2>
          <ul class="list">
            <For each={done()}>{(e) => <Row e={e} />}</For>
          </ul>
        </section>
      </Show>
    </div>
  );
}
