import { For, Show, createSignal, createUniqueId } from "solid-js";
import type { Attendee, AttendeeRole } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { userClient } from "~/data/clients";
import { isEmail, parseIdentity, sameIdentity } from "~/lib/identity";
import { identityLabel, personName } from "~/lib/people";
import { STATUS_LABEL } from "~/lib/eventVisual";
import { effectivePerson } from "~/stores/auth";
import { errorMessage } from "~/transport/csilrpc";

export function attendeeLabel(a: Attendee): string {
  return a.name || (a.identity ? identityLabel(a.identity, a.person) : (a.email ?? "Someone"));
}

/**
 * Add people by LinkKeys identity (user@domain or a handle) or by email.
 * A name that resolves through UserService.lookup-person is added as an
 * identity; otherwise an email-shaped entry is added as an email.
 */
export function AttendeesEditor(props: { value: Attendee[]; onChange: (a: Attendee[]) => void }) {
  const id = createUniqueId();
  const [text, setText] = createSignal("");
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  const add = async () => {
    const raw = text().trim();
    if (!raw) return;
    setError(null);
    const ref = parseIdentity(raw, effectivePerson()?.domain);
    if (!ref && !isEmail(raw)) return setError("Enter someone as user@domain, a handle, or an email address.");
    const list = props.value;
    if (ref && list.some((a) => sameIdentity(a.identity, ref))) return setError("That person is already invited.");
    if (!ref && list.some((a) => a.email?.toLowerCase() === raw.toLowerCase())) return setError("That address is already invited.");
    setBusy(true);
    try {
      let next: Attendee | null = null;
      if (ref) {
        try {
          const person = await userClient.lookupPerson({ identity: ref });
          next = { identity: { domain: person.domain, userId: person.userId, handle: person.handle }, person, name: person.displayName, role: "required", status: "needs_action", rsvp: true };
        } catch (e) {
          if (!isEmail(raw)) throw e;
        }
      }
      if (!next) next = { email: raw, role: "required", status: "needs_action", rsvp: true };
      props.onChange([...list, next]);
      setText("");
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const setRole = (i: number, role: AttendeeRole) => props.onChange(props.value.map((a, j) => (j === i ? { ...a, role } : a)));
  const remove = (i: number) => props.onChange(props.value.filter((_, j) => j !== i));

  return (
    <div class="field">
      <label for={`${id}-add`}>People</label>
      <div class="row">
        <input
          id={`${id}-add`}
          class="input grow"
          placeholder="user@domain or email"
          value={text()}
          autocomplete="off"
          onInput={(e) => setText(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              void add();
            }
          }}
        />
        <button type="button" class="btn" onClick={add} disabled={busy() || !text().trim()}>
          Add
        </button>
      </div>
      <Show when={error()}>
        <p class="alert small" role="alert">
          {error()}
        </p>
      </Show>
      <Show when={props.value.length > 0}>
        <ul class="list attendee-list">
          <For each={props.value}>
            {(a, i) => (
              <li>
                <span class="grow">
                  <span class="truncate" style={{ display: "block" }}>
                    {attendeeLabel(a)}
                  </span>
                  <span class="small muted">
                    {a.identity ? (a.person ? `${a.identity.handle ?? a.person.subject}@${a.identity.domain}` : "catalystcal account") : "by email"}
                    <Show when={a.respondedBy}> · answered by {personName(a.respondedBy)}</Show>
                  </span>
                </span>
                <span class="pill" classList={{ "pill-ok": a.status === "accepted", "pill-danger": a.status === "declined", "pill-warn": a.status === "tentative" }}>
                  {STATUS_LABEL[a.status ?? "needs_action"]}
                </span>
                <select class="select select-sm" aria-label={`Role for ${attendeeLabel(a)}`} value={a.role ?? "required"} onChange={(e) => setRole(i(), e.currentTarget.value as AttendeeRole)}>
                  <option value="required">Required</option>
                  <option value="optional">Optional</option>
                  <option value="chair">Chair</option>
                  <option value="non_participant">For information</option>
                </select>
                <button type="button" class="icon-btn icon-btn-sm" aria-label={`Remove ${attendeeLabel(a)}`} onClick={() => remove(i())}>
                  <Icon name="close" size={14} />
                </button>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </div>
  );
}
