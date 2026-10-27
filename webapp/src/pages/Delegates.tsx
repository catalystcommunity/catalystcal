import { For, Show, createMemo, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { Delegation } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { Modal } from "~/components/Modal";
import { DelegationForm, dateFromExpiry } from "~/components/DelegationForm";
import { delegationClient } from "~/data/clients";
import { isActing, useSession } from "~/stores/auth";
import { calendarById, useCalendars } from "~/stores/calendars";
import { confirmAction } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import { displayZone } from "~/stores/prefs";
import { SCOPE_SHORT, normalizeScopes } from "~/lib/scopes";
import { identityLabel, initials, personName } from "~/lib/people";
import { formatDate } from "~/lib/tz";
import { actAs, stopActing } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";

function state(d: Delegation): "active" | "revoked" | "expired" {
  if (d.revokedAt) return "revoked";
  if (d.expiresAt && Date.parse(d.expiresAt) <= Date.now()) return "expired";
  return "active";
}

function Terms(props: { d: Delegation }) {
  const tz = displayZone();
  return (
    <div class="deleg-terms">
      <div class="row" style={{ gap: "4px" }}>
        <For each={normalizeScopes(props.d.scopes)}>{(s) => <span class="pill pill-accent">{SCOPE_SHORT[s]}</span>}</For>
      </div>
      <p class="small muted">
        {props.d.calendarIds?.length
          ? `Only ${props.d.calendarIds.map((id) => calendarById(id)?.name ?? "a calendar").join(", ")}`
          : "All calendars, now and later"}
        {props.d.expiresAt ? ` · until ${formatDate(dateFromExpiry(props.d.expiresAt, tz), { month: "short", day: "numeric", year: "numeric" })}` : ""}
      </p>
      <Show when={props.d.note}>
        <p class="small">“{props.d.note}”</p>
      </Show>
    </div>
  );
}

export function DelegatesPage() {
  const session = useSession();
  const navigate = useNavigate();
  const cals = useCalendars();
  const mine = createMemo(() => cals().filter((c) => c.owner.userId === session()?.user.userId && c.kind === "local"));
  const [editing, setEditing] = createSignal<Delegation | "new" | null>(null);

  const [granted, { refetch: refetchGranted }] = createResource(
    () => (session() && !isActing() ? session()!.user.userId : false),
    () => delegationClient.listGranted({}),
  );
  const [received] = createResource(
    () => (session() && !isActing() ? session()!.user.userId : false),
    () => delegationClient.listReceived({}),
  );
  const grantedList = () => (granted.error ? [] : (granted.latest?.delegations ?? [])).filter((d) => !d.revokedAt);
  const receivedList = () => (received.error ? [] : (received.latest?.delegations ?? []));

  const revoke = async (d: Delegation) => {
    const ok = await confirmAction({
      title: "Revoke delegate",
      message: `${identityLabel(d.delegate, d.delegatePerson)} can no longer act as you. If they are acting as you now, it stops at once.`,
      confirmLabel: "Revoke",
      danger: true,
    });
    if (!ok) return;
    try {
      await delegationClient.revokeDelegation(d.delegationId);
      toast("Delegate revoked.");
      void refetchGranted();
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  };

  const useDelegation = async (d: Delegation) => {
    try {
      await actAs(d.principal);
      toast(`You are now acting as ${personName(d.principal)}.`);
      navigate("/");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  };

  return (
    <div class="page">
      <header class="page-head-text">
        <h1>Delegates</h1>
        <p class="lede">
          A delegate acts as you: events they make and answers they give are in your name. Use sharing on a calendar instead when someone should work under their own name.
        </p>
      </header>

      <Show when={isActing()}>
        <div class="notice notice-warn" role="note">
          <p>
            You are acting as {personName(session()?.actingAs)}. Only {personName(session()?.actingAs)} can see or change who acts for them.
          </p>
          <div style={{ "margin-top": "8px" }}>
            <button type="button" class="btn btn-sm" onClick={() => stopActing().catch((e) => toast(errorMessage(e), "error"))}>
              Stop acting
            </button>
          </div>
        </div>
      </Show>

      <Show when={!isActing()}>
        <section class="card" aria-labelledby="granted-h">
          <div class="row-between">
            <h2 id="granted-h">People who can act as me</h2>
            <button type="button" class="btn btn-primary" onClick={() => setEditing("new")}>
              <Icon name="plus" /> Add a delegate
            </button>
          </div>
          <Show when={granted.error}>
            <p class="alert" role="alert">
              Delegates did not load: {errorMessage(granted.error)}
            </p>
          </Show>
          <ul class="list deleg-list">
            <For each={grantedList()}>
              {(d) => (
                <li classList={{ "is-expired": state(d) !== "active" }}>
                  <span class="avatar">{initials(identityLabel(d.delegate, d.delegatePerson))}</span>
                  <div class="grow">
                    <p class="deleg-name">
                      {identityLabel(d.delegate, d.delegatePerson)}
                      <Show when={state(d) === "expired"}>
                        <span class="pill" style={{ "margin-left": "6px" }}>
                          Ended
                        </span>
                      </Show>
                    </p>
                    <Terms d={d} />
                  </div>
                  <div class="row deleg-actions">
                    <button type="button" class="btn btn-sm" onClick={() => setEditing(d)}>
                      Change
                    </button>
                    <button type="button" class="btn btn-sm btn-danger" onClick={() => revoke(d)}>
                      Revoke
                    </button>
                  </div>
                </li>
              )}
            </For>
            <Show when={!granted.loading && !granted.error && grantedList().length === 0}>
              <li class="empty-row">
                <p class="muted">Nobody can act as you. Add a delegate to let an assistant or a partner manage your calendar for you.</p>
              </li>
            </Show>
          </ul>
        </section>

        <section class="card" aria-labelledby="recv-h">
          <h2 id="recv-h">People I can act for</h2>
          <Show when={received.error}>
            <p class="alert" role="alert">
              This list did not load: {errorMessage(received.error)}
            </p>
          </Show>
          <ul class="list deleg-list">
            <For each={receivedList()}>
              {(d) => (
                <li classList={{ "is-expired": state(d) !== "active" }}>
                  <span class="avatar avatar-acting">{initials(personName(d.principal))}</span>
                  <div class="grow">
                    <p class="deleg-name">{personName(d.principal)}</p>
                    <Terms d={d} />
                  </div>
                  <Show when={state(d) === "active"} fallback={<span class="pill">{state(d) === "revoked" ? "Revoked" : "Ended"}</span>}>
                    <button type="button" class="btn btn-primary btn-sm" onClick={() => useDelegation(d)}>
                      <Icon name="swap" /> Act as {personName(d.principal)}
                    </button>
                  </Show>
                </li>
              )}
            </For>
            <Show when={!received.loading && !received.error && receivedList().length === 0}>
              <li class="empty-row">
                <p class="muted">Nobody has made you their delegate.</p>
              </li>
            </Show>
          </ul>
        </section>
      </Show>

      <Show when={editing()}>
        {(ed) => (
          <Modal title={ed() === "new" ? "Add a delegate" : "Change delegate"} onClose={() => setEditing(null)}>
            <DelegationForm
              calendars={mine()}
              initial={ed() === "new" ? undefined : (ed() as Delegation)}
              onCancel={() => setEditing(null)}
              onSaved={() => {
                const wasNew = ed() === "new";
                setEditing(null);
                toast(wasNew ? "Delegate added." : "Delegate changed.");
                void refetchGranted();
              }}
            />
          </Modal>
        )}
      </Show>
    </div>
  );
}
