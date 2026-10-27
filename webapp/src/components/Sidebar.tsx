import { For, Show, createMemo, createSignal } from "solid-js";
import { A, useLocation, useNavigate } from "@solidjs/router";
import { Icon } from "~/components/Icons";
import { MiniMonth } from "~/components/MiniMonth";
import { ImportDialog, NewCalendarDialog, SubscribeDialog } from "~/components/CalendarDialogs";
import { calendarColor, toggleHidden, useCalendars, useCalendarsError, useCalendarsLoaded, useHidden, accessAtLeast } from "~/stores/calendars";
import { effectivePerson, hasScope, isActing } from "~/stores/auth";
import { groupCalendars } from "~/lib/calendarGroups";
import { goToDate, setSidebarOpen, useCursor, useView } from "~/stores/nav";
import { useWeekStart, displayZone } from "~/stores/prefs";
import { visibleRange } from "~/lib/view";

export const NAV = [
  { href: "/", label: "Calendar", icon: "calendar" as const },
  { href: "/invitations", label: "Invitations", icon: "inbox" as const },
  { href: "/delegates", label: "Delegates", icon: "users" as const },
  { href: "/activity", label: "Activity", icon: "activity" as const },
];

export function Sidebar() {
  const cals = useCalendars();
  const loaded = useCalendarsLoaded();
  const error = useCalendarsError();
  const hidden = useHidden();
  const cursor = useCursor();
  const view = useView();
  const weekStart = useWeekStart();
  const navigate = useNavigate();
  const location = useLocation();
  const [dialog, setDialog] = createSignal<"new" | "subscribe" | "import" | null>(null);

  const groups = createMemo(() => groupCalendars(cals(), effectivePerson(), isActing()));
  const rangeDays = createMemo(() => (view() === "month" || view() === "agenda" ? [] : visibleRange(view(), cursor(), displayZone(), weekStart()).days));
  const canImport = () => cals().some((c) => c.kind === "local" && accessAtLeast(c.access, "write"));

  return (
    <nav class="sidebar" aria-label="Calendars and navigation">
      <div class="sidebar-nav">
        <For each={NAV}>
          {(n) => (
            <A href={n.href} end class="side-link" activeClass="is-active" onClick={() => setSidebarOpen(false)}>
              <Icon name={n.icon} size={16} />
              {n.label}
            </A>
          )}
        </For>
      </div>

      <MiniMonth
        selected={cursor()}
        rangeDays={rangeDays()}
        onPick={(d) => {
          goToDate(d);
          setSidebarOpen(false);
          if (location.pathname !== "/") navigate("/");
        }}
      />

      <Show when={error()}>
        <p class="alert small" role="alert">
          Calendars did not load: {error()}
        </p>
      </Show>

      <For each={groups()}>
        {(g) => (
          <section class="cal-group" aria-labelledby={`grp-${g.key}`}>
            <h3 id={`grp-${g.key}`} class="cal-group-title" classList={{ "is-acting": g.label.startsWith("Acting as") }}>
              {g.label}
            </h3>
            <ul class="cal-list">
              <For each={g.calendars}>
                {(c) => (
                  <li class="cal-item">
                    <label class="cal-toggle">
                      <input type="checkbox" checked={!hidden().has(c.calendarId)} onChange={() => toggleHidden(c.calendarId)} aria-label={`Show ${c.name}`} />
                      <span class="cal-box" style={{ "--sw": calendarColor(c) }} aria-hidden="true">
                        <Icon name="check" size={12} />
                      </span>
                      <span class="cal-name truncate">{c.name}</span>
                    </label>
                    <Show when={c.syncError}>
                      <span class="pill pill-danger" title={c.syncError}>
                        sync
                      </span>
                    </Show>
                    <A href={`/calendars/${encodeURIComponent(c.calendarId)}`} class="icon-btn icon-btn-sm cal-gear" aria-label={`Settings for ${c.name}`} onClick={() => setSidebarOpen(false)}>
                      <Icon name="settings" size={14} />
                    </A>
                  </li>
                )}
              </For>
              <Show when={g.calendars.length === 0 && loaded()}>
                <li class="small muted cal-empty">No calendars yet.</li>
              </Show>
            </ul>
          </section>
        )}
      </For>

      <div class="side-actions">
        <button type="button" class="side-action" onClick={() => setDialog("new")} disabled={isActing() && !hasScope("calendars.manage")}>
          <Icon name="plus" size={16} /> New calendar
        </button>
        <button type="button" class="side-action" onClick={() => setDialog("subscribe")} disabled={isActing() && !hasScope("calendars.manage")}>
          <Icon name="link" size={16} /> Subscribe to URL
        </button>
        <button type="button" class="side-action" onClick={() => setDialog("import")} disabled={!canImport()}>
          <Icon name="upload" size={16} /> Import .ics
        </button>
      </div>

      <Show when={dialog() === "new"}>
        <NewCalendarDialog onClose={() => setDialog(null)} />
      </Show>
      <Show when={dialog() === "subscribe"}>
        <SubscribeDialog onClose={() => setDialog(null)} />
      </Show>
      <Show when={dialog() === "import"}>
        <ImportDialog onClose={() => setDialog(null)} />
      </Show>
    </nav>
  );
}
