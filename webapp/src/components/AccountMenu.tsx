import { For, Show, createSignal, onCleanup } from "solid-js";
import { A, useNavigate } from "@solidjs/router";
import type { PersonSummary } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { effectivePerson, isActing, useMe, useSession } from "~/stores/auth";
import { initials, personAddress, personName, samePerson } from "~/lib/people";
import { actAs, endSession, stopActing } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";

export function AccountMenu() {
  const session = useSession();
  const me = useMe();
  const navigate = useNavigate();
  const [open, setOpen] = createSignal(false);
  let wrap!: HTMLDivElement;
  let button!: HTMLButtonElement;

  const onDoc = (e: MouseEvent) => {
    if (open() && !wrap.contains(e.target as Node)) setOpen(false);
  };
  const onKey = (e: KeyboardEvent) => {
    if (open() && e.key === "Escape") {
      setOpen(false);
      button.focus();
    }
  };
  document.addEventListener("mousedown", onDoc);
  document.addEventListener("keydown", onKey);
  onCleanup(() => {
    document.removeEventListener("mousedown", onDoc);
    document.removeEventListener("keydown", onKey);
  });

  const principals = () => (me()?.principals ?? []).filter((p) => !samePerson(p, session()?.user));

  const switchTo = async (p: PersonSummary | null) => {
    setOpen(false);
    try {
      if (isActing()) await stopActing();
      if (p) {
        await actAs(p);
        toast(`You are now acting as ${personName(p)}.`);
      }
      navigate("/");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  };

  const signOut = async () => {
    setOpen(false);
    await endSession();
    navigate("/login", { replace: true });
  };

  return (
    <div class="menu-wrap" ref={wrap}>
      <button
        ref={button}
        type="button"
        class="account-btn"
        aria-haspopup="menu"
        aria-expanded={open()}
        aria-label={`Account: ${personName(effectivePerson())}`}
        onClick={() => setOpen(!open())}
      >
        <span class="avatar" classList={{ "avatar-acting": isActing() }}>
          {initials(personName(effectivePerson()))}
        </span>
        <Icon name="chevronDown" size={14} />
      </button>
      <Show when={open()}>
        <div class="menu" role="menu">
          <div class="menu-account">
            <div class="truncate" style={{ "font-weight": 600 }}>
              {personName(session()?.user)}
            </div>
            <div class="small muted truncate">{personAddress(session()?.user)}</div>
          </div>
          <div class="menu-sep" />
          <div class="menu-heading">Use catalystcal as</div>
          <button type="button" role="menuitemradio" aria-checked={!isActing()} class="menu-item" onClick={() => switchTo(null)}>
            <span class="avatar">{initials(personName(session()?.user))}</span>
            <span class="grow truncate">Yourself</span>
            <Show when={!isActing()}>
              <Icon name="check" />
            </Show>
          </button>
          <For each={principals()}>
            {(p) => (
              <button
                type="button"
                role="menuitemradio"
                aria-checked={samePerson(p, session()?.actingAs)}
                class="menu-item"
                onClick={() => (samePerson(p, session()?.actingAs) ? setOpen(false) : switchTo(p))}
              >
                <span class="avatar avatar-acting">{initials(personName(p))}</span>
                <span class="grow truncate">{personName(p)}</span>
                <Show when={samePerson(p, session()?.actingAs)}>
                  <Icon name="check" />
                </Show>
              </button>
            )}
          </For>
          <Show when={principals().length === 0}>
            <p class="small muted" style={{ padding: "2px 10px 6px" }}>
              Nobody has let you act for them yet.
            </p>
          </Show>
          <div class="menu-sep" />
          <A href="/settings" class="menu-item" role="menuitem" onClick={() => setOpen(false)}>
            <Icon name="settings" /> Settings
          </A>
          <A href="/delegates" class="menu-item" role="menuitem" onClick={() => setOpen(false)}>
            <Icon name="users" /> Delegates
          </A>
          <button type="button" class="menu-item" role="menuitem" onClick={signOut}>
            <Icon name="logout" /> Sign out
          </button>
        </div>
      </Show>
    </div>
  );
}
