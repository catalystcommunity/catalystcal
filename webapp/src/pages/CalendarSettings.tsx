import { For, Show, createEffect, createResource, createSignal } from "solid-js";
import { A, useNavigate, useParams } from "@solidjs/router";
import type { Calendar, Share, ShareRole } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { ColorPicker } from "~/components/ColorPicker";
import { ImportIcsForm } from "~/components/CalendarDialogs";
import { calendarClient, icsClient, shareClient } from "~/data/clients";
import { accessAtLeast, bumpEvents, calendarById, calendarColor, removeCalendar, upsertCalendar } from "~/stores/calendars";
import { effectivePerson, isActing } from "~/stores/auth";
import { confirmAction } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import { displayZone } from "~/stores/prefs";
import { parseIdentity } from "~/lib/identity";
import { identityLabel, personName, samePerson } from "~/lib/people";
import { copyText, downloadText } from "~/lib/download";
import { zoneList } from "~/lib/tz";
import { errorMessage } from "~/transport/csilrpc";

export const ROLE_LABEL: Record<ShareRole, string> = {
  free_busy: "See only free/busy",
  read: "See all event details",
  write: "Make changes to events",
  manage: "Make changes and manage sharing",
};

const ACCESS_LABEL: Record<string, string> = {
  owner: "You own this calendar",
  manage: "You can change events and manage sharing",
  write: "You can change events",
  read: "You can see event details",
  free_busy: "You can see only free/busy time",
  none: "You have no access",
};

export function feedUrls(feedPath: string, origin = window.location.origin): { https: string; webcal: string } {
  const https = new URL(feedPath, origin).toString();
  return { https, webcal: https.replace(/^https?:/, "webcal:") };
}

export function CalendarSettingsPage() {
  const params = useParams();
  const navigate = useNavigate();
  const [cal, { mutate }] = createResource(
    () => params.id,
    (id) => calendarClient.getCalendar(id),
  );
  const current = (): Calendar | undefined => (cal.error ? calendarById(params.id ?? "") : (cal.latest ?? calendarById(params.id ?? "")));
  const canManage = () => accessAtLeast(current()?.access, "manage");
  const canWrite = () => accessAtLeast(current()?.access, "write");
  const isOwner = () => current()?.access === "owner";

  const apply = (c: Calendar) => {
    mutate(c);
    upsertCalendar(c);
  };

  return (
    <div class="page">
      <A href="/" class="back-link">
        <Icon name="chevronLeft" size={16} /> Calendar
      </A>
      <Show when={cal.error && !current()}>
        <p class="alert" role="alert">
          This calendar did not load: {errorMessage(cal.error)}
        </p>
      </Show>
      <Show when={current()}>
        {(c) => (
          <>
            <header class="page-head">
              <span class="swatch swatch-lg" style={{ "--sw": calendarColor(c()) }} />
              <div>
                <h1>{c().name}</h1>
                <p class="muted">
                  {ACCESS_LABEL[c().access] ?? c().access}
                  <Show when={!samePerson(c().owner, effectivePerson())}> · owned by {personName(c().owner)}</Show>
                  <Show when={c().accessPath === "delegation"}> · through a delegation</Show>
                </p>
              </div>
            </header>

            <Show when={c().kind === "subscription"}>
              <SubscriptionSection cal={c()} onChange={apply} canManage={canManage()} />
            </Show>

            <GeneralSection cal={c()} canManage={canManage()} onSaved={apply} />

            <Show when={canManage() && c().kind === "local"}>
              <SharingSection cal={c()} />
            </Show>

            <Show when={canManage() && c().kind === "local"}>
              <FeedSection cal={c()} onChange={apply} />
            </Show>

            <section class="card" aria-labelledby="exp-h">
              <h2 id="exp-h">Export</h2>
              <p class="muted">Download every event in this calendar as an iCalendar (.ics) file.</p>
              <div>
                <button
                  type="button"
                  class="btn"
                  onClick={async () => {
                    try {
                      const doc = await icsClient.exportCalendar({ calendarId: c().calendarId });
                      downloadText(doc.filename || `${c().name}.ics`, doc.ics);
                    } catch (e) {
                      toast(errorMessage(e), "error");
                    }
                  }}
                >
                  <Icon name="download" /> Export .ics
                </button>
              </div>
            </section>

            <Show when={canWrite() && c().kind === "local"}>
              <section class="card" aria-labelledby="imp-h">
                <h2 id="imp-h">Import</h2>
                <ImportIcsForm calendarId={c().calendarId} />
              </section>
            </Show>

            <Show when={isOwner() && !isActing()}>
              <section class="card card-danger" aria-labelledby="del-h">
                <h2 id="del-h">Delete calendar</h2>
                <Show when={!c().isDefault} fallback={<p class="muted">This is your default calendar. You cannot delete it.</p>}>
                  <p class="muted">This removes the calendar and every event in it. People it is shared with lose access.</p>
                  <div>
                    <button
                      type="button"
                      class="btn btn-danger"
                      onClick={async () => {
                        const ok = await confirmAction({ title: "Delete calendar", message: `Delete “${c().name}” and all of its events? You cannot undo this.`, confirmLabel: "Delete calendar", danger: true });
                        if (!ok) return;
                        try {
                          await calendarClient.deleteCalendar(c().calendarId);
                          removeCalendar(c().calendarId);
                          bumpEvents();
                          toast(`Deleted “${c().name}”.`);
                          navigate("/", { replace: true });
                        } catch (e) {
                          toast(errorMessage(e), "error");
                        }
                      }}
                    >
                      <Icon name="trash" /> Delete calendar
                    </button>
                  </div>
                </Show>
              </section>
            </Show>
            <Show when={cal.loading}>
              <span class="sr-only" role="status">
                Loading
              </span>
            </Show>
          </>
        )}
      </Show>
    </div>
  );
}

function GeneralSection(props: { cal: Calendar; canManage: boolean; onSaved: (c: Calendar) => void }) {
  const [name, setName] = createSignal(props.cal.name);
  const [color, setColor] = createSignal(props.cal.color ?? "");
  const [tz, setTz] = createSignal(props.cal.timezone ?? displayZone());
  const [description, setDescription] = createSignal(props.cal.description ?? "");
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);
  createEffect(() => {
    setName(props.cal.name);
    setColor(props.cal.color ?? "");
    setTz(props.cal.timezone ?? displayZone());
    setDescription(props.cal.description ?? "");
  });

  const save = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!name().trim()) return setError("Give the calendar a name.");
    setBusy(true);
    setError(null);
    try {
      const c = await calendarClient.updateCalendar({ calendarId: props.cal.calendarId, name: name().trim(), color: color() || undefined, timezone: tz(), description: description() });
      props.onSaved(c);
      toast("Calendar saved.");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section class="card" aria-labelledby="gen-h">
      <h2 id="gen-h">Details</h2>
      <form class="stack" onSubmit={save}>
        <div class="grid-2">
          <div class="field">
            <label for="cs-name">Name</label>
            <input id="cs-name" class="input" maxLength={256} value={name()} disabled={!props.canManage} onInput={(e) => setName(e.currentTarget.value)} />
          </div>
          <div class="field">
            <label for="cs-tz">Time zone</label>
            <select id="cs-tz" class="select" value={tz()} disabled={!props.canManage} onChange={(e) => setTz(e.currentTarget.value)}>
              <For each={zoneList(tz())}>{(z) => <option value={z}>{z.replace(/_/g, " ")}</option>}</For>
            </select>
          </div>
        </div>
        <div class="field">
          <span class="field-label">Color</span>
          <Show when={props.canManage} fallback={<span class="swatch swatch-lg" style={{ "--sw": calendarColor(props.cal) }} />}>
            <ColorPicker label="Calendar color" value={color()} onChange={setColor} />
          </Show>
        </div>
        <div class="field">
          <label for="cs-desc">Description</label>
          <textarea id="cs-desc" class="textarea" value={description()} disabled={!props.canManage} onInput={(e) => setDescription(e.currentTarget.value)} />
        </div>
        <Show when={error()}>
          <p class="alert" role="alert">
            {error()}
          </p>
        </Show>
        <Show when={props.canManage}>
          <div>
            <button type="submit" class="btn btn-primary" disabled={busy()}>
              Save details
            </button>
          </div>
        </Show>
      </form>
    </section>
  );
}

function SubscriptionSection(props: { cal: Calendar; canManage: boolean; onChange: (c: Calendar) => void }) {
  const [busy, setBusy] = createSignal(false);
  return (
    <section class="card" aria-labelledby="sub-h">
      <h2 id="sub-h">Subscription</h2>
      <p class="small muted">catalystcal reads this address again from time to time. You cannot change these events here.</p>
      <p class="mono small break">{props.cal.sourceUrl}</p>
      <p class="small">
        {props.cal.lastSyncedAt ? `Last read ${new Date(props.cal.lastSyncedAt).toLocaleString()}` : "Not read yet."}
      </p>
      <Show when={props.cal.syncError}>
        <p class="alert small" role="alert">
          The last read failed: {props.cal.syncError}
        </p>
      </Show>
      <Show when={props.canManage}>
        <div>
          <button
            type="button"
            class="btn"
            disabled={busy()}
            onClick={async () => {
              setBusy(true);
              try {
                props.onChange(await calendarClient.refreshSubscription(props.cal.calendarId));
                bumpEvents();
                toast("Subscription read again.");
              } catch (e) {
                toast(errorMessage(e), "error");
              } finally {
                setBusy(false);
              }
            }}
          >
            <Icon name="repeat" /> Read it now
          </button>
        </div>
      </Show>
    </section>
  );
}

function SharingSection(props: { cal: Calendar }) {
  const [shares, { refetch, mutate }] = createResource(
    () => props.cal.calendarId,
    (id) => shareClient.listShares(id),
  );
  const [who, setWho] = createSignal("");
  const [role, setRole] = createSignal<ShareRole>("read");
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);
  const list = (): Share[] => (shares.error ? [] : (shares.latest?.shares ?? []));

  const add = async (e: SubmitEvent) => {
    e.preventDefault();
    const ref = parseIdentity(who(), effectivePerson()?.domain);
    if (!ref) return setError("Enter the person as user@domain (their handle or account id at their LinkKeys domain).");
    setBusy(true);
    setError(null);
    try {
      await shareClient.putShare({ shareId: "", calendarId: props.cal.calendarId, grantee: ref, role: role(), createdAt: "" });
      setWho("");
      await refetch();
      toast("Shared.");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const change = async (s: Share, r: ShareRole) => {
    try {
      const updated = await shareClient.putShare({ shareId: s.shareId, calendarId: s.calendarId, grantee: s.grantee, role: r, createdAt: s.createdAt });
      mutate({ shares: list().map((x) => (x.shareId === s.shareId ? updated : x)) });
      toast("Access changed.");
    } catch (err) {
      toast(errorMessage(err), "error");
      void refetch();
    }
  };

  const remove = async (s: Share) => {
    const ok = await confirmAction({ title: "Stop sharing", message: `Stop sharing “${props.cal.name}” with ${identityLabel(s.grantee, s.granteePerson)}?`, confirmLabel: "Stop sharing", danger: true });
    if (!ok) return;
    try {
      await shareClient.deleteShare(s.shareId);
      mutate({ shares: list().filter((x) => x.shareId !== s.shareId) });
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  };

  return (
    <section class="card" aria-labelledby="share-h">
      <h2 id="share-h">Sharing</h2>
      <p class="muted">People you share with work under their own name. To let someone act as you, add a delegate on the Delegates page.</p>
      <Show when={shares.error}>
        <p class="alert" role="alert">
          Shares did not load: {errorMessage(shares.error)}
        </p>
      </Show>
      <ul class="list">
        <For each={list()}>
          {(s) => (
            <li>
              <span class="grow">
                <span class="truncate" style={{ display: "block" }}>
                  {identityLabel(s.grantee, s.granteePerson)}
                </span>
                <Show when={!s.granteePerson}>
                  <span class="small muted">Has not signed in to catalystcal yet</span>
                </Show>
              </span>
              <select class="select select-sm share-role" aria-label={`Access for ${identityLabel(s.grantee, s.granteePerson)}`} value={s.role} onChange={(e) => change(s, e.currentTarget.value as ShareRole)}>
                <For each={Object.entries(ROLE_LABEL)}>{([v, l]) => <option value={v}>{l}</option>}</For>
              </select>
              <button type="button" class="icon-btn" aria-label={`Stop sharing with ${identityLabel(s.grantee, s.granteePerson)}`} onClick={() => remove(s)}>
                <Icon name="trash" size={16} />
              </button>
            </li>
          )}
        </For>
        <Show when={!shares.loading && !shares.error && list().length === 0}>
          <li class="muted small">Not shared with anyone.</li>
        </Show>
      </ul>
      <form class="share-add" onSubmit={add}>
        <div class="field grow">
          <label for="sh-who">Share with</label>
          <input id="sh-who" class="input" placeholder="user@domain" value={who()} onInput={(e) => setWho(e.currentTarget.value)} />
        </div>
        <div class="field">
          <label for="sh-role">Access</label>
          <select id="sh-role" class="select" value={role()} onChange={(e) => setRole(e.currentTarget.value as ShareRole)}>
            <For each={Object.entries(ROLE_LABEL)}>{([v, l]) => <option value={v}>{l}</option>}</For>
          </select>
        </div>
        <button type="submit" class="btn btn-primary" disabled={busy() || !who().trim()}>
          Share
        </button>
      </form>
      <Show when={error()}>
        <p class="alert" role="alert">
          {error()}
        </p>
      </Show>
    </section>
  );
}

function FeedSection(props: { cal: Calendar; onChange: (c: Calendar) => void }) {
  const [busy, setBusy] = createSignal(false);
  const urls = () => (props.cal.feedPath ? feedUrls(props.cal.feedPath) : null);

  const setFeed = async (enabled: boolean) => {
    if (enabled && props.cal.feedPath) {
      const ok = await confirmAction({ title: "Make a new address", message: "The current address stops working. Apps that use it stop getting updates.", confirmLabel: "Make a new address" });
      if (!ok) return;
    }
    setBusy(true);
    try {
      props.onChange(await calendarClient.setFeed({ calendarId: props.cal.calendarId, enabled }));
      toast(enabled ? "Secret address ready." : "Secret address turned off.");
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section class="card" aria-labelledby="feed-h">
      <h2 id="feed-h">Secret address (iCalendar feed)</h2>
      <p class="muted">Apple Calendar, Google Calendar, and Outlook can subscribe to this address and show your events.</p>
      <label class="check">
        <input type="checkbox" checked={!!props.cal.feedPath} disabled={busy()} onChange={(e) => setFeed(e.currentTarget.checked)} />
        Publish a secret address
      </label>
      <Show when={urls()}>
        {(u) => (
          <>
            <p class="notice notice-warn" role="note">
              <strong>Keep this address secret.</strong> Anyone who has it can see the events in this calendar, without signing in.
            </p>
            <div class="row feed-row">
              <label for="feed-url" class="sr-only">
                Secret address
              </label>
              <input id="feed-url" class="input mono grow" readonly value={u().https} onFocus={(e) => e.currentTarget.select()} />
              <button
                type="button"
                class="btn"
                onClick={async () => {
                  toast((await copyText(u().https)) ? "Address copied." : "Copy did not work. Select the address and copy it.", "info");
                }}
              >
                <Icon name="copy" /> Copy
              </button>
            </div>
            <div class="row">
              <a class="btn" href={u().webcal}>
                <Icon name="calendar" /> Open in calendar app
              </a>
              <button type="button" class="btn" disabled={busy()} onClick={() => setFeed(true)}>
                <Icon name="repeat" /> Make a new address
              </button>
            </div>
          </>
        )}
      </Show>
    </section>
  );
}
