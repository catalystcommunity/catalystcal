import { For } from "solid-js";
import { useToasts } from "~/stores/toast";

export function Toasts() {
  const toasts = useToasts();
  return (
    <div class="toasts" aria-live="polite">
      <For each={toasts()}>
        {(t) => (
          <div class="toast" classList={{ "toast-error": t.kind === "error" }} role={t.kind === "error" ? "alert" : "status"}>
            {t.text}
          </div>
        )}
      </For>
    </div>
  );
}
