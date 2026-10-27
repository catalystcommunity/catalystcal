import { JSX, Show, createEffect, onCleanup } from "solid-js";
import { A, Navigate, useLocation } from "@solidjs/router";
import { Icon } from "~/components/Icons";
import { Sidebar } from "~/components/Sidebar";
import { SearchBox } from "~/components/SearchBox";
import { AccountMenu } from "~/components/AccountMenu";
import { ActingBanner } from "~/components/ActingBanner";
import { GlobalDialogs } from "~/components/GlobalDialogs";
import { EventEditor } from "~/components/EventEditor";
import { EventDetail } from "~/components/EventDetail";
import { isActing, useRestoring, useSession } from "~/stores/auth";
import { setSidebarOpen, useSidebarOpen } from "~/stores/nav";
import { closeDetail, closeEditor, useDetail, useEditor } from "~/stores/editor";
import { rememberAuthReturnTo } from "~/lib/session";

/** The signed-in frame: top bar, banner, sidebar, page, global dialogs. */
export function AppShell(props: { children?: JSX.Element }) {
  const session = useSession();
  const restoring = useRestoring();
  const location = useLocation();
  const open = useSidebarOpen();
  const editor = useEditor();
  const detail = useDetail();

  createEffect(() => {
    // Close the drawer when the route changes.
    location.pathname;
    setSidebarOpen(false);
  });

  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape" && open()) setSidebarOpen(false);
  };
  document.addEventListener("keydown", onKey);
  onCleanup(() => document.removeEventListener("keydown", onKey));

  return (
    <Show
      when={session()}
      fallback={
        <Show
          when={restoring()}
          fallback={(() => {
            rememberAuthReturnTo(location.pathname + location.search);
            return <Navigate href="/login" />;
          })()}
        >
          <div class="center-page" role="status">
            <div class="spinner" aria-hidden="true" />
            <p>Signing you in…</p>
          </div>
        </Show>
      }
    >
      <div class="app" classList={{ "is-acting": isActing(), "drawer-open": open() }}>
        <a class="skip-link" href="#main">
          Skip to content
        </a>
        <ActingBanner />
        <header class="topbar">
          <button type="button" class="icon-btn drawer-btn" aria-label={open() ? "Close menu" : "Open menu"} aria-expanded={open()} aria-controls="sidebar" onClick={() => setSidebarOpen(!open())}>
            <Icon name="menu" />
          </button>
          <A href="/" class="brand" aria-label="CatalystCal, calendar">
            <svg class="brand-mark" viewBox="0 0 24 24" aria-hidden="true">
              <rect x="2.5" y="4" width="19" height="17" rx="4" />
              <circle cx="15.5" cy="14.5" r="2.4" />
            </svg>
            <span>CatalystCal</span>
          </A>
          <SearchBox />
          <div class="topbar-end">
            <AccountMenu />
          </div>
        </header>
        <div class="body">
          <div id="sidebar" class="sidebar-wrap" classList={{ open: open() }}>
            <Sidebar />
          </div>
          <Show when={open()}>
            <div class="drawer-scrim" onClick={() => setSidebarOpen(false)} aria-hidden="true" />
          </Show>
          <main id="main" class="main" tabindex="-1">
            {props.children}
          </main>
        </div>
        <Show when={editor()}>{(s) => <EventEditor state={s()} onClose={closeEditor} />}</Show>
        <Show when={detail()}>{(o) => <EventDetail occ={o()} onClose={closeDetail} />}</Show>
        <GlobalDialogs />
      </div>
    </Show>
  );
}
