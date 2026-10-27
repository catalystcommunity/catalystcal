/**
 * Dialog with a focus trap. Escape and a scrim click close it; focus goes
 * back to the element that opened it.
 */
import { JSX, Show, createUniqueId, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { Icon } from "~/components/Icons";

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

// Only the top dialog handles Escape and Tab.
const stack: HTMLElement[] = [];

export interface ModalProps {
  title: string;
  onClose: () => void;
  children: JSX.Element;
  footer?: JSX.Element;
  /** "dialog" (centered) or "sheet" (from the right edge). */
  variant?: "dialog" | "sheet";
  wide?: boolean;
  /** Hide the visible heading (it is still the accessible name). */
  hideTitle?: boolean;
  headExtra?: JSX.Element;
  /** Element to focus first; else the first field, else the close button. */
  initialFocus?: () => HTMLElement | undefined;
  role?: "dialog" | "alertdialog";
}

export function Modal(props: ModalProps) {
  let panel!: HTMLDivElement;
  const titleId = createUniqueId();
  const opener = document.activeElement as HTMLElement | null;

  const onKey = (e: KeyboardEvent) => {
    if (stack[stack.length - 1] !== panel) return;
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      props.onClose();
      return;
    }
    if (e.key !== "Tab") return;
    const items = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.offsetParent !== null || el === document.activeElement);
    if (!items.length) {
      e.preventDefault();
      panel.focus();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && (document.activeElement === first || document.activeElement === panel)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  };

  onMount(() => {
    stack.push(panel);
    document.addEventListener("keydown", onKey, true);
    queueMicrotask(() => {
      const target =
        props.initialFocus?.() ??
        panel.querySelector<HTMLElement>("[data-autofocus]") ??
        panel.querySelector<HTMLElement>(".dialog-body " + FOCUSABLE) ??
        panel;
      target.focus();
    });
  });

  onCleanup(() => {
    const i = stack.indexOf(panel);
    if (i >= 0) stack.splice(i, 1);
    document.removeEventListener("keydown", onKey, true);
    if (opener && document.contains(opener)) opener.focus();
  });

  const sheet = () => props.variant === "sheet";

  return (
    <Portal>
      <div
        class="scrim"
        classList={{ "scrim-sheet": sheet() }}
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) props.onClose();
        }}
      >
        <div
          ref={panel}
          class="dialog"
          classList={{ sheet: sheet(), "dialog-wide": !!props.wide }}
          role={props.role ?? "dialog"}
          aria-modal="true"
          aria-labelledby={titleId}
          tabindex="-1"
        >
          <div class="dialog-head">
            <h2 id={titleId} classList={{ "sr-only": !!props.hideTitle }}>
              {props.title}
            </h2>
            <div class="row" style={{ "margin-left": "auto" }}>
              {props.headExtra}
              <button type="button" class="icon-btn" aria-label="Close" onClick={() => props.onClose()}>
                <Icon name="close" />
              </button>
            </div>
          </div>
          <div class="dialog-body">{props.children}</div>
          <Show when={props.footer}>
            <div class="dialog-foot">{props.footer}</div>
          </Show>
        </div>
      </div>
    </Portal>
  );
}
