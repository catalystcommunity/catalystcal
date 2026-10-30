/**
 * /auth/callback: LinkKeys sends the browser here with a sealed
 * `encrypted_token`. AuthService.complete opens it and returns our bearer.
 * `domain` is the identity domain that /api/v1/auth/start put on the
 * callback URL.
 */
import { Show, createSignal, onMount } from "solid-js";
import { useNavigate, useSearchParams } from "@solidjs/router";
import { authClient } from "~/data/clients";
import { finishLogin, takeAuthReturnTo } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";

export function AuthCallback() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [error, setError] = createSignal<string | null>(null);

  onMount(async () => {
    const token = params.encrypted_token;
    if (typeof token !== "string" || token === "") {
      setError(typeof params.error === "string" ? params.error : "The identity provider did not send a sign-in token.");
      return;
    }
    try {
      const domain = typeof params.domain === "string" && params.domain !== "" ? params.domain : undefined;
      const resp = await authClient.complete({ encryptedToken: token, domain });
      await finishLogin(resp);
      navigate(takeAuthReturnTo(), { replace: true });
    } catch (e) {
      setError(errorMessage(e));
    }
  });

  return (
    <div class="center-page">
      <Show
        when={error()}
        fallback={
          <div class="center-card" role="status">
            <div class="spinner" aria-hidden="true" />
            <p>Signing you in…</p>
          </div>
        }
      >
        {(msg) => (
          <div class="center-card">
            <h1>Sign-in did not finish</h1>
            <p class="alert" role="alert">
              {msg()}
            </p>
            <a class="btn btn-primary" href="/login">
              Try again
            </a>
          </div>
        )}
      </Show>
    </div>
  );
}
