import { For, Match, Show, Switch, createSignal } from "solid-js";
import type { RecurrenceScope } from "@catalystcal/client";
import { Modal } from "~/components/Modal";
import { ConfirmRequest, ScopeRequest, clearPendingDialog, usePendingDialog } from "~/stores/dialogs";
import { Toasts } from "~/components/Toasts";

export function GlobalDialogs() {
  const pending = usePendingDialog();
  return (
    <>
      <Switch>
        <Match when={pending()?.kind === "confirm" && (pending() as ConfirmRequest)}>{(req) => <ConfirmDialog req={req()} />}</Match>
        <Match when={pending()?.kind === "scope" && (pending() as ScopeRequest)}>{(req) => <ScopeDialog req={req()} />}</Match>
      </Switch>
      <Toasts />
    </>
  );
}

function ConfirmDialog(props: { req: ConfirmRequest }) {
  // Read the request before clearPendingDialog: clearing unmounts the
  // <Match>, and a later read of props.req throws "Stale read".
  const done = (ok: boolean) => {
    const { resolve } = props.req;
    clearPendingDialog();
    resolve(ok);
  };
  return (
    <Modal
      title={props.req.title}
      role="alertdialog"
      onClose={() => done(false)}
      footer={
        <>
          <button type="button" class="btn" onClick={() => done(false)}>
            Cancel
          </button>
          <button type="button" class="btn" classList={{ "btn-primary": !props.req.danger, "btn-danger": !!props.req.danger }} onClick={() => done(true)} data-autofocus>
            {props.req.confirmLabel}
          </button>
        </>
      }
    >
      <p>{props.req.message}</p>
    </Modal>
  );
}

const SCOPES: { value: RecurrenceScope; label: string }[] = [
  { value: "this", label: "This event" },
  { value: "this_and_future", label: "This and following events" },
  { value: "all", label: "All events" },
];

function ScopeDialog(props: { req: ScopeRequest }) {
  const [scope, setScope] = createSignal<RecurrenceScope>("this");
  const done = (s: RecurrenceScope | null) => {
    const { resolve } = props.req;
    clearPendingDialog();
    resolve(s);
  };
  const verb = () => (props.req.action === "delete" ? "Delete" : "Save");
  return (
    <Modal
      title={props.req.action === "delete" ? "Delete a repeating event" : "Change a repeating event"}
      onClose={() => done(null)}
      footer={
        <>
          <button type="button" class="btn" onClick={() => done(null)}>
            Cancel
          </button>
          <button type="button" class="btn" classList={{ "btn-primary": props.req.action === "save", "btn-danger": props.req.action === "delete" }} onClick={() => done(scope())}>
            {verb()}
          </button>
        </>
      }
    >
      <fieldset class="scope-options">
        <legend class="sr-only">Which events</legend>
        <For each={SCOPES}>
          {(s) => (
            <label class="check scope-option">
              <input type="radio" name="scope" value={s.value} checked={scope() === s.value} onChange={() => setScope(s.value)} />
              {s.label}
            </label>
          )}
        </For>
      </fieldset>
      <Show when={scope() === "all" && props.req.action === "save"}>
        <p class="hint">A time change moves every event in the series by the same amount.</p>
      </Show>
    </Modal>
  );
}
