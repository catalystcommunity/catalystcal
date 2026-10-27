import { For, Show, createSignal, createUniqueId } from "solid-js";
import type { Calendar, Delegation, Scope } from "@catalystcal/client";
import { delegationClient } from "~/data/clients";
import { SCOPES, normalizeScopes } from "~/lib/scopes";
import { formatIdentity, parseIdentity } from "~/lib/identity";
import { identityLabel } from "~/lib/people";
import { addDays, isDateText } from "~/lib/dates";
import { civilDate, parseTimestamp, toTimestamp, todayIn, zonedToUtc } from "~/lib/tz";
import { useSession } from "~/stores/auth";
import { displayZone } from "~/stores/prefs";
import { errorMessage } from "~/transport/csilrpc";

/** End of `date` in `tz`, as the delegation expiry instant. */
export function expiryFromDate(date: string, tz: string): string {
  return toTimestamp(zonedToUtc(addDays(date, 1), 0, tz));
}
export function dateFromExpiry(ts: string, tz: string): string {
  return civilDate(parseTimestamp(ts) - 1000, tz);
}

export interface DelegationFormProps {
  /** The principal's own calendars, for the optional restriction. */
  calendars: Calendar[];
  initial?: Delegation;
  onSaved: (d: Delegation) => void;
  onCancel: () => void;
}

export function DelegationForm(props: DelegationFormProps) {
  const id = createUniqueId();
  const session = useSession();
  const tz = displayZone();
  const init = props.initial;
  const [who, setWho] = createSignal(init ? formatIdentity(init.delegate) : "");
  const [scopes, setScopes] = createSignal<Scope[]>(init ? normalizeScopes(init.scopes) : ["calendars.read", "freebusy.read"]);
  const [restrict, setRestrict] = createSignal(!!init?.calendarIds?.length);
  const [calIds, setCalIds] = createSignal<string[]>(init?.calendarIds ?? []);
  const [expires, setExpires] = createSignal(init?.expiresAt ? dateFromExpiry(init.expiresAt, tz) : "");
  const [note, setNote] = createSignal(init?.note ?? "");
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  const toggleScope = (s: Scope, on: boolean) => {
    // Unchecking "details" leaves "busy times" checked: the smaller grant.
    setScopes(normalizeScopes(on ? [...scopes(), s] : scopes().filter((x) => x !== s)));
  };
  const toggleCal = (cid: string, on: boolean) => setCalIds(on ? [...calIds(), cid] : calIds().filter((x) => x !== cid));

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    setError(null);
    const self = session()?.user;
    const delegate = init ? init.delegate : parseIdentity(who(), self?.domain);
    if (!delegate) return setError("Enter the delegate as user@domain (their handle or account id at their LinkKeys domain).");
    if (self && delegate.domain === self.domain.toLowerCase() && (delegate.userId === self.userId || (delegate.handle && delegate.handle === self.handle))) {
      return setError("You cannot make yourself your own delegate.");
    }
    if (scopes().length === 0) return setError("Choose at least one thing this person can do.");
    if (restrict() && calIds().length === 0) return setError("Choose at least one calendar, or allow all calendars.");
    if (expires() && (!isDateText(expires()) || expires() < todayIn(tz))) return setError("The end date must be today or later.");

    const body: Delegation = {
      delegationId: init?.delegationId ?? "",
      principal: init?.principal ?? self ?? { userId: "", domain: "", subject: "" },
      delegate,
      scopes: normalizeScopes(scopes()),
      calendarIds: restrict() ? calIds() : [],
      note: note().trim() || undefined,
      expiresAt: expires() ? expiryFromDate(expires(), tz) : undefined,
      createdAt: init?.createdAt ?? "",
    };
    setBusy(true);
    try {
      const saved = init ? await delegationClient.updateDelegation(body) : await delegationClient.createDelegation(body);
      props.onSaved(saved);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form class="stack" onSubmit={submit} aria-label={init ? "Change delegate" : "Add a delegate"}>
      <div class="field">
        <label for={`${id}-who`}>Delegate</label>
        <Show when={!init} fallback={<p>{identityLabel(init!.delegate, init!.delegatePerson)}</p>}>
          <input id={`${id}-who`} class="input" placeholder="user@domain" value={who()} onInput={(e) => setWho(e.currentTarget.value)} autocomplete="off" data-autofocus />
          <span class="hint">Their LinkKeys handle or account id and domain, for example sam@example.com.</span>
        </Show>
      </div>

      <fieldset class="field scope-set">
        <legend class="field-label">This person can</legend>
        <For each={SCOPES}>
          {(s) => {
            const implied = () => s.scope === "freebusy.read" && scopes().includes("calendars.read");
            return (
              <label class="check scope-check">
                <input
                  type="checkbox"
                  name="scope"
                  value={s.scope}
                  checked={scopes().includes(s.scope)}
                  disabled={implied()}
                  onChange={(e) => toggleScope(s.scope, e.currentTarget.checked)}
                />
                <span>
                  <span>{s.label}</span>
                  <span class="hint" style={{ display: "block" }}>
                    {implied() ? "Included in “See my calendars and event details”." : s.detail}
                  </span>
                </span>
              </label>
            );
          }}
        </For>
      </fieldset>

      <fieldset class="field">
        <legend class="field-label">Calendars</legend>
        <label class="check">
          <input type="radio" name={`${id}-restrict`} checked={!restrict()} onChange={() => setRestrict(false)} />
          All my calendars, now and later
        </label>
        <label class="check">
          <input type="radio" name={`${id}-restrict`} checked={restrict()} onChange={() => setRestrict(true)} />
          Only these calendars
        </label>
        <Show when={restrict()}>
          <div class="cal-checks">
            <For each={props.calendars}>
              {(c) => (
                <label class="check">
                  <input type="checkbox" checked={calIds().includes(c.calendarId)} onChange={(e) => toggleCal(c.calendarId, e.currentTarget.checked)} />
                  <span class="swatch" style={{ "--sw": c.color || "var(--accent)" }} />
                  {c.name}
                </label>
              )}
            </For>
          </div>
          <span class="hint">Invitations to you are not on a calendar. “Answer invitations as me” still applies to them.</span>
        </Show>
      </fieldset>

      <div class="grid-2">
        <div class="field">
          <label for={`${id}-exp`}>Ends on (optional)</label>
          <input id={`${id}-exp`} class="input" type="date" min={todayIn(tz)} value={expires()} onInput={(e) => setExpires(e.currentTarget.value)} />
          <span class="hint">Access stops at the end of this day.</span>
        </div>
        <div class="field">
          <label for={`${id}-note`}>Note (optional)</label>
          <input id={`${id}-note`} class="input" placeholder="Covering while I am on leave" value={note()} onInput={(e) => setNote(e.currentTarget.value)} />
        </div>
      </div>

      <Show when={error()}>
        <p class="alert" role="alert">
          {error()}
        </p>
      </Show>
      <div class="row" style={{ "justify-content": "flex-end" }}>
        <button type="button" class="btn" onClick={props.onCancel}>
          Cancel
        </button>
        <button type="submit" class="btn btn-primary" disabled={busy()}>
          {init ? "Save changes" : "Add delegate"}
        </button>
      </div>
    </form>
  );
}
