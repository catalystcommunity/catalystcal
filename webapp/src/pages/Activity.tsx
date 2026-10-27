import { For, Show, createResource, createSignal } from "solid-js";
import type { AuditEntry } from "@catalystcal/client";
import { auditClient } from "~/data/clients";
import { useSession } from "~/stores/auth";
import { displayZone } from "~/stores/prefs";
import { civilDate, formatDate, formatTime, parseTimestamp } from "~/lib/tz";
import { personName, samePerson } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";

const VIA: Record<string, string> = { self: "Directly", delegation: "As a delegate", act_as_grant: "Through an app" };

/** "EventService/create-event" or "create-event" -> "Create event". */
export function operationLabel(op: string): string {
  const last = op.split(/[/.]/).pop() ?? op;
  const words = last.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/[-_]/g, " ").toLowerCase();
  return words.charAt(0).toUpperCase() + words.slice(1);
}

export function ActivityPage() {
  const session = useSession();
  const [onlyOthers, setOnlyOthers] = createSignal(false);
  const [entries] = createResource(
    () => (session() ? { delegated: onlyOthers(), who: session()!.actingAs?.userId ?? session()!.user.userId } : false),
    (k) => auditClient.listAudit({ limit: 200, delegatedOnly: k.delegated }),
  );
  const list = (): AuditEntry[] => (entries.error ? [] : (entries.latest?.entries ?? []));
  const tz = () => displayZone();
  const when = (e: AuditEntry) => {
    const ms = parseTimestamp(e.at);
    return `${formatDate(civilDate(ms, tz()), { month: "short", day: "numeric", year: "numeric" })}, ${formatTime(ms, tz())}`;
  };

  return (
    <div class="page">
      <header class="page-head-text">
        <h1>Activity</h1>
        <p class="lede">Every change made in your name, and who made it. A delegate’s changes show their own name here.</p>
      </header>
      <section class="card">
        <div class="row-between">
          <label class="check">
            <input type="checkbox" checked={onlyOthers()} onChange={(e) => setOnlyOthers(e.currentTarget.checked)} />
            Only actions others took as me
          </label>
          <Show when={entries.loading}>
            <span class="spinner spinner-sm" role="status" aria-label="Loading" />
          </Show>
        </div>
        <Show when={entries.error}>
          <p class="alert" role="alert">
            Activity did not load: {errorMessage(entries.error)}
          </p>
        </Show>
        <div class="table-wrap">
          <table class="table activity-table">
            <thead>
              <tr>
                <th scope="col">When</th>
                <th scope="col">Who</th>
                <th scope="col">For</th>
                <th scope="col">How</th>
                <th scope="col">What</th>
                <th scope="col">Result</th>
              </tr>
            </thead>
            <tbody>
              <For each={list()}>
                {(e) => (
                  <tr classList={{ "is-delegated": !samePerson(e.actor, e.principal) || e.via !== "self" }}>
                    <td class="tnum nowrap">{when(e)}</td>
                    <td>{personName(e.actor)}</td>
                    <td>{samePerson(e.actor, e.principal) ? <span class="muted">themselves</span> : personName(e.principal)}</td>
                    <td>
                      <span class="pill" classList={{ "pill-warn": e.via !== "self" }}>
                        {VIA[e.via] ?? e.via}
                      </span>
                      <Show when={e.application}>
                        <span class="small muted"> {e.application}</span>
                      </Show>
                    </td>
                    <td>
                      {operationLabel(e.operation)}
                      <Show when={e.target}>
                        <span class="small muted mono break"> {e.target}</span>
                      </Show>
                    </td>
                    <td>
                      <span class="pill" classList={{ "pill-ok": e.outcome === "ok", "pill-danger": e.outcome !== "ok" }}>
                        {e.outcome}
                      </span>
                    </td>
                  </tr>
                )}
              </For>
            </tbody>
          </table>
        </div>
        <Show when={!entries.loading && !entries.error && list().length === 0}>
          <p class="empty">{onlyOthers() ? "Nobody has acted as you." : "No activity yet."}</p>
        </Show>
      </section>
    </div>
  );
}
