/**
 * Session flows over the generated AuthService client. A browser sign-in
 * also starts a server session in an httpOnly cookie; resumeSession trades
 * the cookie for a new bearer so the app stays signed in.
 */
import type { LoginResponse, PersonSummary } from "@catalystcal/client";
import { authClient } from "~/data/clients";
import { sessionFromLogin, setMe, signIn, signOut, storedSession, useSession } from "~/stores/auth";
import { clearCalendars, loadCalendars } from "~/stores/calendars";
import { bumpEvents } from "~/stores/calendars";
import { setUnauthenticatedHandler } from "~/transport/csilrpc";

const AUTH_RETURN_KEY = "catalystcal.auth.return_to";
const RESUME_LOCK = "catalystcal.session.resume";
/** A bearer with less than this left is renewed. */
export const REFRESH_AHEAD_MS = 5 * 60 * 1000;

/** Store the session from any login response, then load /me and calendars. */
export async function finishLogin(resp: LoginResponse): Promise<void> {
  signIn(sessionFromLogin(resp));
  await loadMe();
  void loadCalendars();
  bumpEvents();
}

export async function loadMe(): Promise<void> {
  try {
    setMe(await authClient.me({}));
  } catch {
    // /me failure leaves the session as it is; pages show their own errors.
  }
}

/** Use a delegation: get a bearer that acts as `principal`. */
export async function actAs(principal: PersonSummary): Promise<void> {
  const resp = await authClient.assumePrincipal({ principalUserId: principal.userId });
  await finishLogin(resp);
}

/** Stop acting as somebody: get a self bearer again. */
export async function stopActing(): Promise<void> {
  const resp = await authClient.releasePrincipal({});
  await finishLogin(resp);
}

const fresh = (expiresAt: string | undefined): boolean => {
  const ms = expiresAt ? Date.parse(expiresAt) - Date.now() : NaN;
  return !Number.isNaN(ms) && ms > REFRESH_AHEAD_MS;
};

let inFlight: Promise<boolean> | null = null;
let resuming = false;

/**
 * Trade the session cookie for a new bearer. One request per tab; a Web
 * Lock serializes tabs because the cookie rotates on each use.
 */
export function resumeSession(): Promise<boolean> {
  if (!inFlight) {
    inFlight = withResumeLock(resumeLocked).finally(() => {
      inFlight = null;
    });
  }
  return inFlight;
}

async function resumeLocked(): Promise<boolean> {
  resuming = true;
  try {
    const current = useSession()();
    const stored = storedSession();
    if (stored && fresh(stored.expiresAt) && stored.token !== current?.token) {
      signIn(stored);
      return true;
    }
    try {
      const resp = await authClient.resumeSession({});
      signIn(sessionFromLogin(resp));
      void loadMe();
      return true;
    } catch {
      const s = useSession()();
      if (s && Date.parse(s.expiresAt) > Date.now() + 5000) return false;
      if (s) {
        signOut();
        clearCalendars();
      }
      return false;
    }
  } finally {
    resuming = false;
  }
}

function withResumeLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== "undefined" ? navigator.locks : undefined;
  return locks ? (locks.request(RESUME_LOCK, fn) as Promise<T>) : fn();
}

/** End the server session (clears the cookie), then drop local state. */
export async function endSession(): Promise<void> {
  try {
    await authClient.endSession({});
  } catch {
    // Sign out here anyway; the server session expires on its own.
  }
  signOut();
  clearCalendars();
}

// A request that comes back unauthenticated gets one resume and one retry.
setUnauthenticatedHandler(() => (resuming ? Promise.resolve(false) : resumeSession()));

/** Remember one same-origin route across the LinkKeys redirect. */
export function rememberAuthReturnTo(path: string): void {
  try {
    const target = new URL(path, window.location.origin);
    if (target.origin !== window.location.origin || target.pathname.startsWith("/auth/") || target.pathname === "/login") return;
    sessionStorage.setItem(AUTH_RETURN_KEY, `${target.pathname}${target.search}${target.hash}`);
  } catch {
    // Ignore; the callback goes to the calendar.
  }
}

export function takeAuthReturnTo(): string {
  let target = "/";
  try {
    target = sessionStorage.getItem(AUTH_RETURN_KEY) ?? "/";
    sessionStorage.removeItem(AUTH_RETURN_KEY);
  } catch {
    // sessionStorage is not available.
  }
  return target.startsWith("/") && !target.startsWith("//") ? target : "/";
}

export const AUTH_START_URL = "/api/v1/auth/start";

/** Leave the app for the LinkKeys sign-in. */
export function startLinkKeysLogin(returnTo?: string): void {
  if (returnTo) rememberAuthReturnTo(returnTo);
  window.location.assign(AUTH_START_URL);
}
