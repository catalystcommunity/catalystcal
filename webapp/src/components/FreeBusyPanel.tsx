/**
 * "Find a time": busy blocks for you and each attendee with a catalystcal
 * identity, side by side for one day, with the proposed time drawn across.
 */
import { For, Show, createMemo, createResource, createSignal, createEffect } from "solid-js";
import type { Attendee, IdentityRef, PersonFreeBusy } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { freeBusyClient } from "~/data/clients";
import { effectivePerson } from "~/stores/auth";
import { DateText, addDays } from "~/lib/dates";
import { formatDate, formatHourLabel, minutesIntoDay, parseTimestamp, startOfDayUtc, toTimestamp } from "~/lib/tz";
import { personName } from "~/lib/people";
import { attendeeLabel } from "~/components/AttendeesEditor";
import { errorMessage } from "~/transport/csilrpc";

const FROM = 6;
const TO = 22;

export function FreeBusyPanel(props: { attendees: Attendee[]; date: DateText; startMin: number; endMin: number; endDate: DateText; tz: string; allDay: boolean; onPick?: (startMin: number) => void }) {
  const [day, setDay] = createSignal(props.date);
  createEffect(() => setDay(props.date));

  const people = createMemo(() => {
    const me = effectivePerson();
    const list: { ref: IdentityRef; label: string }[] = [];
    if (me) list.push({ ref: { domain: me.domain, userId: me.userId }, label: `${personName(me)} (you)` });
    for (const a of props.attendees) if (a.identity && (a.identity.userId || a.identity.handle)) list.push({ ref: a.identity, label: attendeeLabel(a) });
    return list;
  });
  const key = () => JSON.stringify({ d: day(), p: people().map((p) => p.ref), tz: props.tz });

  const [data] = createResource(key, async () => {
    if (people().length === 0) return [] as PersonFreeBusy[];
    const start = startOfDayUtc(day(), props.tz);
    const end = startOfDayUtc(addDays(day(), 1), props.tz);
    const r = await freeBusyClient.query({ people: people().map((p) => p.ref), rangeStart: toTimestamp(start), rangeEnd: toTimestamp(end) });
    return r.people ?? [];
  });

  const result = (i: number): PersonFreeBusy | undefined => (data.error ? undefined : data.latest?.[i]);
  const pct = (min: number) => `${((Math.min(TO * 60, Math.max(FROM * 60, min)) - FROM * 60) / ((TO - FROM) * 60)) * 100}%`;

  const blocks = (pf: PersonFreeBusy | undefined) => {
    if (!pf) return [];
    const ds = startOfDayUtc(day(), props.tz);
    const de = startOfDayUtc(addDays(day(), 1), props.tz);
    return pf.busy
      .map((b) => ({ s: Math.max(ds, parseTimestamp(b.start)), e: Math.min(de, parseTimestamp(b.end)), tentative: !!b.tentative }))
      .filter((b) => b.e > b.s)
      .map((b) => ({ top: b.s <= ds ? 0 : minutesIntoDay(b.s, props.tz), bottom: b.e >= de ? 1440 : minutesIntoDay(b.e, props.tz), tentative: b.tentative }));
  };

  const proposal = () => {
    if (props.allDay || day() !== props.date) return null;
    const end = props.endDate === props.date ? props.endMin : 1440;
    return { top: props.startMin, bottom: end };
  };

  return (
    <section class="fb" aria-labelledby="fb-h">
      <div class="row-between">
        <h3 id="fb-h">Find a time</h3>
        <div class="row" style={{ gap: "2px" }}>
          <button type="button" class="icon-btn icon-btn-sm" aria-label="Previous day" onClick={() => setDay(addDays(day(), -1))}>
            <Icon name="chevronLeft" size={14} />
          </button>
          <span class="small">{formatDate(day(), { weekday: "short", month: "short", day: "numeric" })}</span>
          <button type="button" class="icon-btn icon-btn-sm" aria-label="Next day" onClick={() => setDay(addDays(day(), 1))}>
            <Icon name="chevronRight" size={14} />
          </button>
        </div>
      </div>
      <Show when={data.error}>
        <p class="alert small" role="alert">
          Free and busy times did not load: {errorMessage(data.error)}
        </p>
      </Show>
      <div class="fb-grid" style={{ "--cols": people().length }}>
        <div class="fb-hours" aria-hidden="true">
          <For each={Array.from({ length: TO - FROM }, (_, i) => FROM + i)}>{(h) => <div class="fb-hour tnum">{formatHourLabel(h)}</div>}</For>
        </div>
        <For each={people()}>
          {(p, i) => (
            <div class="fb-col" classList={{ "is-hidden": result(i())?.visible === false }}>
              <div class="fb-name truncate" title={result(i())?.person ? personName(result(i())!.person) : p.label}>
                {result(i())?.person ? personName(result(i())!.person) : p.label}
              </div>
              <div
                class="fb-track"
                role="img"
                aria-label={result(i())?.visible === false ? `${p.label}: time not shared with you` : `${p.label}: ${blocks(result(i())).length} busy blocks`}
                onClick={(e) => {
                  if (!props.onPick || props.allDay) return;
                  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
                  const m = FROM * 60 + Math.round((((e.clientY - r.top) / r.height) * (TO - FROM) * 60) / 30) * 30;
                  if (day() === props.date) props.onPick(m);
                }}
              >
                <Show when={result(i())?.visible === false}>
                  <span class="fb-hidden-label">Time not shared</span>
                </Show>
                <For each={blocks(result(i()))}>
                  {(b) => <div class="fb-busy" classList={{ tentative: b.tentative }} style={{ top: pct(b.top), height: `calc(${pct(b.bottom)} - ${pct(b.top)})` }} />}
                </For>
                <Show when={proposal()}>
                  {(pr) => <div class="fb-proposal" style={{ top: pct(pr().top), height: `calc(${pct(pr().bottom)} - ${pct(pr().top)})` }} />}
                </Show>
              </div>
            </div>
          )}
        </For>
      </div>
      <p class="hint">
        Hatched columns are people whose time you cannot see. <Show when={props.onPick && !props.allDay}>Click a column to move the event there.</Show>
      </p>
    </section>
  );
}

