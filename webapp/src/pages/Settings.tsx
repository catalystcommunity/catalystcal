import { For, Show, createEffect, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { userClient } from "~/data/clients";
import { isActing, useMe, useSession } from "~/stores/auth";
import { ThemeChoice, setTheme, useTheme } from "~/stores/theme";
import { setWeekStart, useWeekStart } from "~/stores/prefs";
import { toast } from "~/stores/toast";
import { endSession, loadMe } from "~/lib/session";
import { browserZone, zoneList } from "~/lib/tz";
import { personAddress } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";

export function SettingsPage() {
  const session = useSession();
  const me = useMe();
  const theme = useTheme();
  const weekStart = useWeekStart();
  const navigate = useNavigate();
  const [profile, { mutate }] = createResource(
    () => (session() ? session()!.token : false),
    () => userClient.getProfile({}),
  );
  const [name, setName] = createSignal("");
  const [tz, setTz] = createSignal(browserZone());
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  createEffect(() => {
    const p = profile.error ? undefined : profile.latest;
    if (p) {
      setName(p.displayName ?? "");
      setTz(p.timezone || browserZone());
    }
  });

  const save = async (e: SubmitEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const u = await userClient.updateProfile({ displayName: name().trim() || undefined, timezone: tz() });
      mutate(u);
      await loadMe();
      toast("Profile saved.");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const THEMES: { v: ThemeChoice; l: string }[] = [
    { v: "light", l: "Light" },
    { v: "dark", l: "Dark" },
    { v: "system", l: "Match system" },
  ];

  return (
    <div class="page">
      <header class="page-head-text">
        <h1>Settings</h1>
      </header>

      <section class="card" aria-labelledby="prof-h">
        <h2 id="prof-h">Profile</h2>
        <Show when={isActing()}>
          <p class="notice">You are acting as someone else. A delegate cannot change their profile. These are your own settings: stop acting to change them.</p>
        </Show>
        <Show when={profile.error}>
          <p class="alert" role="alert">
            Your profile did not load: {errorMessage(profile.error)}
          </p>
        </Show>
        <p class="small muted">Signed in as {personAddress(session()?.user)}</p>
        <form class="stack" onSubmit={save}>
          <div class="grid-2">
            <div class="field">
              <label for="pf-name">Display name</label>
              <input id="pf-name" class="input" value={name()} disabled={isActing()} onInput={(e) => setName(e.currentTarget.value)} />
            </div>
            <div class="field">
              <label for="pf-tz">Time zone</label>
              <select id="pf-tz" class="select" value={tz()} disabled={isActing()} onChange={(e) => setTz(e.currentTarget.value)}>
                <For each={zoneList(tz())}>{(z) => <option value={z}>{z.replace(/_/g, " ")}</option>}</For>
              </select>
              <span class="hint">The calendar shows times in this zone. This device is in {browserZone().replace(/_/g, " ")}.</span>
            </div>
          </div>
          <Show when={error()}>
            <p class="alert" role="alert">
              {error()}
            </p>
          </Show>
          <div>
            <button type="submit" class="btn btn-primary" disabled={busy() || isActing()}>
              Save profile
            </button>
          </div>
        </form>
      </section>

      <section class="card" aria-labelledby="look-h">
        <h2 id="look-h">On this device</h2>
        <div class="field">
          <span class="field-label" id="theme-l">
            Theme
          </span>
          <div class="segmented" role="radiogroup" aria-labelledby="theme-l">
            <For each={THEMES}>
              {(t) => (
                <button type="button" role="radio" aria-checked={theme() === t.v} onClick={() => setTheme(t.v)}>
                  {t.l}
                </button>
              )}
            </For>
          </div>
        </div>
        <div class="field">
          <span class="field-label" id="wk-l">
            Weeks start on
          </span>
          <div class="segmented" role="radiogroup" aria-labelledby="wk-l">
            <button type="button" role="radio" aria-checked={weekStart() === 0} onClick={() => setWeekStart(0)}>
              Sunday
            </button>
            <button type="button" role="radio" aria-checked={weekStart() === 1} onClick={() => setWeekStart(1)}>
              Monday
            </button>
          </div>
        </div>
        <p class="hint">
          Keyboard: T today, ← → or J K to move, M W D A for views, C for a new event. Version {__APP_VERSION__}.
        </p>
      </section>

      <section class="card" aria-labelledby="out-h">
        <h2 id="out-h">Sign out</h2>
        <p class="muted">Ends your catalystcal session on this browser.</p>
        <div>
          <button
            type="button"
            class="btn btn-danger"
            onClick={async () => {
              await endSession();
              navigate("/login", { replace: true });
            }}
          >
            Sign out
          </button>
        </div>
      </section>
    </div>
  );
}
