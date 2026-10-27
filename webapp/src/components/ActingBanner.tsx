import { Show, createSignal } from "solid-js";
import { isActing, useSession } from "~/stores/auth";
import { personName } from "~/lib/people";
import { stopActing } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";

/** Shown on every page while a delegated bearer is in use. */
export function ActingBanner() {
  const session = useSession();
  const [busy, setBusy] = createSignal(false);
  const stop = async () => {
    setBusy(true);
    try {
      await stopActing();
      toast("You are acting as yourself again.");
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      setBusy(false);
    }
  };
  return (
    <Show when={isActing()}>
      <section class="acting-banner" role="region" aria-label="Acting as another person">
        <div class="acting-text">
          <strong>Acting as {personName(session()?.actingAs)}</strong>
          <span> — changes are made in their name, and they can see what you did</span>
        </div>
        <button type="button" class="btn btn-sm acting-stop" onClick={stop} disabled={busy()}>
          Stop acting
        </button>
      </section>
    </Show>
  );
}
