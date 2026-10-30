import { For, Show, createSignal, onMount } from "solid-js";
import { Navigate, useNavigate, useSearchParams } from "@solidjs/router";
import type { DevUser } from "@catalystcal/client";
import { devAuthClient } from "~/data/clients";
import { finishLogin, rememberedLoginName, setRememberedLoginName, startLinkKeysLogin, takeAuthReturnTo } from "~/lib/session";
import { useSession } from "~/stores/auth";
import { errorMessage } from "~/transport/csilrpc";

/** The decorative week on the sign-in page. */
function WeekArt() {
  return (
    <div class="login-art" aria-hidden="true">
      <div class="art-grid">
        <div class="art-col">
          <div class="art-ev" style={{ "--ev": "#4f6bd8", top: "12%", height: "14%" }} />
          <div class="art-ev" style={{ "--ev": "#2f9e8f", top: "46%", height: "22%" }} />
        </div>
        <div class="art-col">
          <div class="art-ev art-busy" style={{ top: "20%", height: "26%" }} />
          <div class="art-ev" style={{ "--ev": "#d9822b", top: "58%", height: "10%" }} />
        </div>
        <div class="art-col is-today">
          <div class="art-ev" style={{ "--ev": "#4f6bd8", top: "8%", height: "12%", width: "48%" }} />
          <div class="art-ev art-invite" style={{ "--ev": "#8a5cd6", top: "8%", height: "20%", left: "52%", width: "46%" }} />
          <div class="art-ev" style={{ "--ev": "#c2415d", top: "62%", height: "16%" }} />
          <div class="art-now" style={{ top: "44%" }} />
        </div>
        <div class="art-col">
          <div class="art-ev" style={{ "--ev": "#2f9e8f", top: "30%", height: "32%" }} />
        </div>
        <div class="art-col">
          <div class="art-ev art-busy" style={{ top: "14%", height: "12%" }} />
          <div class="art-ev" style={{ "--ev": "#4f6bd8", top: "50%", height: "14%" }} />
        </div>
      </div>
    </div>
  );
}

export function DevLoginPanel() {
  const navigate = useNavigate();
  const [users, setUsers] = createSignal<DevUser[] | null>(null);
  const [handle, setHandle] = createSignal("");
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  onMount(async () => {
    try {
      const r = await devAuthClient.listDevUsers({});
      setUsers(r.users ?? []);
    } catch {
      setUsers(null); // No dev service on this server.
    }
  });

  const login = async (req: { userId?: string; handle?: string }) => {
    setBusy(true);
    setError(null);
    try {
      const resp = await devAuthClient.devLogin(req);
      await finishLogin(resp);
      navigate(takeAuthReturnTo(), { replace: true });
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Show when={users()}>
      {(list) => (
        <section class="dev-panel" aria-labelledby="dev-h">
          <h2 id="dev-h">Development sign-in</h2>
          <p class="small muted">This server runs with dev sign-in turned on. Do not use it with real data.</p>
          <Show when={list().length > 0}>
            <ul class="dev-users">
              <For each={list()}>
                {(u) => (
                  <li>
                    <button type="button" class="btn dev-user" disabled={busy()} onClick={() => login({ userId: u.userId })}>
                      <span class="grow truncate" style={{ "text-align": "left" }}>
                        {u.displayName || u.handle || u.userId}
                      </span>
                      <span class="small muted">
                        {u.handle ?? u.userId}@{u.domain}
                      </span>
                    </button>
                  </li>
                )}
              </For>
            </ul>
          </Show>
          <form
            class="row"
            onSubmit={(e) => {
              e.preventDefault();
              if (handle().trim()) void login({ handle: handle().trim() });
            }}
          >
            <label for="dev-handle" class="sr-only">
              Handle
            </label>
            <input id="dev-handle" class="input grow" placeholder="Handle for a new dev user" value={handle()} onInput={(e) => setHandle(e.currentTarget.value)} />
            <button type="submit" class="btn" disabled={busy() || !handle().trim()}>
              Sign in as dev user
            </button>
          </form>
          <Show when={error()}>
            <p class="alert" role="alert">
              {error()}
            </p>
          </Show>
        </section>
      )}
    </Show>
  );
}

/** The LinkKeys sign-in: a login name such as you@example.com. */
function LinkKeysLoginForm() {
  const [params] = useSearchParams();
  const fromQuery = typeof params.identity === "string" ? params.identity : "";
  const saved = rememberedLoginName();
  const [name, setName] = createSignal(fromQuery || saved);
  const [remember, setRemember] = createSignal(saved !== "");
  const loginError = () => (typeof params.login_error === "string" ? params.login_error : "");

  return (
    <form
      class="login-form"
      onSubmit={(e) => {
        e.preventDefault();
        const value = name().trim();
        if (!value) return;
        setRememberedLoginName(value, remember());
        startLinkKeysLogin(value);
      }}
    >
      <label for="login-name" class="field-label">
        LinkKeys login name
      </label>
      <input
        id="login-name"
        class="input"
        type="text"
        name="identity"
        autocomplete="username"
        autocapitalize="off"
        spellcheck={false}
        placeholder="you@example.com"
        value={name()}
        onInput={(e) => setName(e.currentTarget.value)}
      />
      <label class="row small login-remember">
        <input type="checkbox" checked={remember()} onChange={(e) => setRemember(e.currentTarget.checked)} />
        Remember my login name
      </label>
      <Show when={loginError()}>
        <p class="alert" role="alert">
          {loginError()}
        </p>
      </Show>
      <button type="submit" class="btn btn-primary btn-lg login-btn" disabled={!name().trim()}>
        Sign in with LinkKeys
      </button>
    </form>
  );
}

export function LoginPage() {
  const session = useSession();
  return (
    <Show when={!session()} fallback={<Navigate href="/" />}>
      <div class="login">
        <WeekArt />
        <main class="login-panel">
          <div class="login-brand">
            <svg class="brand-mark" viewBox="0 0 24 24" aria-hidden="true">
              <rect x="2.5" y="4" width="19" height="17" rx="4" />
              <circle cx="15.5" cy="14.5" r="2.4" />
            </svg>
            CatalystCal
          </div>
          <h1 class="login-title">Your calendar, and the people you let into it.</h1>
          <p class="login-lede">
            Share a calendar so someone can work with it as themselves, or let a person act for you. Every change made in your name shows in your activity.
          </p>
          <LinkKeysLoginForm />
          <p class="small muted">You sign in at your own LinkKeys domain. CatalystCal does not see your password.</p>
          <DevLoginPanel />
        </main>
      </div>
    </Show>
  );
}
