import { For, Show, createMemo } from "solid-js";
import type { Occurrence } from "@catalystcal/client";
import { DateText } from "~/lib/dates";
import { Placed, covers } from "~/lib/occurrence";
import { formatDate, formatTime, todayIn } from "~/lib/tz";
import { STATUS_LABEL, myStatus, occClasses, occColor, occTitle } from "~/lib/eventVisual";
import { calendarById } from "~/stores/calendars";
import { useNow } from "~/stores/clock";
import { Icon } from "~/components/Icons";

interface Props {
  days: DateText[];
  placed: Placed[];
  tz: string;
  onOpen: (o: Occurrence) => void;
  onPickDay: (d: DateText) => void;
}

export function AgendaView(props: Props) {
  const now = useNow();
  const today = () => todayIn(props.tz, now());
  const groups = createMemo(() =>
    props.days
      .map((d) => ({ date: d, items: props.placed.filter((p) => covers(p, d)) }))
      .filter((g) => g.items.length > 0 || g.date === today()),
  );

  const when = (p: Placed, d: DateText) => {
    if (p.allDay) return "All day";
    if (p.startDate < d && p.lastDate > d) return "All day";
    if (p.startDate < d) return `Until ${formatTime(p.endMs, props.tz)}`;
    if (p.lastDate > d) return `${formatTime(p.startMs, props.tz)} →`;
    return `${formatTime(p.startMs, props.tz)} – ${formatTime(p.endMs, props.tz)}`;
  };

  return (
    <div class="agenda">
      <Show when={groups().length > 0} fallback={<p class="empty">Nothing scheduled in the next 30 days.</p>}>
        <For each={groups()}>
          {(g) => (
            <section class="agenda-day" classList={{ "is-today": g.date === today() }} aria-labelledby={`ag-${g.date}`}>
              <h2 id={`ag-${g.date}`} class="agenda-date">
                <button type="button" class="link-btn" onClick={() => props.onPickDay(g.date)}>
                  <span class="agenda-dnum tnum">{Number(g.date.slice(8))}</span>
                  <span class="agenda-dlabel">
                    <span>{formatDate(g.date, { weekday: "long" })}</span>
                    <span class="muted small">{formatDate(g.date, { month: "long", year: "numeric" })}</span>
                  </span>
                </button>
              </h2>
              <ul class="agenda-list">
                <Show when={g.items.length === 0}>
                  <li class="muted small agenda-none">Nothing today.</li>
                </Show>
                <For each={g.items}>
                  {(p) => (
                    <li>
                      <button type="button" class="agenda-item" classList={occClasses(p.occ)} style={{ "--ev": occColor(p.occ) }} onClick={() => props.onOpen(p.occ)}>
                        <span class="agenda-time tnum">{when(p, g.date)}</span>
                        <span class="agenda-bar" aria-hidden="true" />
                        <span class="agenda-main">
                          <span class="agenda-title">{occTitle(p.occ)}</span>
                          <span class="agenda-sub small muted">
                            <Show when={!p.occ.busyOnly && p.occ.event.location}>
                              <span class="row-inline">
                                <Icon name="pin" size={12} /> {p.occ.event.location}
                              </span>
                            </Show>
                            <span>{calendarById(p.occ.event.calendarId)?.name ?? (p.occ.invited ? "Invitation" : "")}</span>
                            <Show when={p.occ.event.recurrence || p.occ.recurrenceId}>
                              <Icon name="repeat" size={12} />
                            </Show>
                          </span>
                        </span>
                        <Show when={p.occ.invited}>
                          <span class="pill" classList={{ "pill-ok": myStatus(p.occ) === "accepted", "pill-warn": !myStatus(p.occ) || myStatus(p.occ) === "needs_action" || myStatus(p.occ) === "tentative", "pill-danger": myStatus(p.occ) === "declined" }}>
                            {STATUS_LABEL[myStatus(p.occ) ?? "needs_action"]}
                          </span>
                        </Show>
                      </button>
                    </li>
                  )}
                </For>
              </ul>
            </section>
          )}
        </For>
      </Show>
    </div>
  );
}
