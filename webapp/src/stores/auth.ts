/**
 * Auth store: the bearer, who it acts for, and the /me answer. Pure state;
 * the network calls that fill it live in lib/session.ts (the transport
 * imports this module, so it must not import the clients).
 *
 * The bearer lives in memory and in localStorage. A delegated bearer (via =
 * "delegation") acts AS another person; the UI shows a banner while it is in
 * use.
 */
import { cell } from "~/state/store";
import type { ActingVia, LoginResponse, MeResponse, PersonSummary, Scope } from "@catalystcal/client";
import { readStore, writeStore } from "~/lib/storage";

export const SESSION_KEY = "catalystcal.session";

export interface Session {
  token: string;
  expiresAt: string;
  /** The person who signed in (the real actor). */
  user: PersonSummary;
  /** Set while a delegated bearer acts for another person. */
  actingAs?: PersonSummary;
  via: ActingVia;
  scopes?: Scope[];
}

export function sessionFromLogin(r: LoginResponse): Session {
  return { token: r.token, expiresAt: r.expiresAt, user: r.user, actingAs: r.actingAs, via: r.via, scopes: r.scopes };
}

const readSession = (): Session | null => {
  const raw = readStore(SESSION_KEY);
  if (!raw) return null;
  try {
    const s = JSON.parse(raw) as Session;
    if (!s.token) return null;
    if (s.expiresAt && Date.parse(s.expiresAt) < Date.now()) return null;
    return s;
  } catch {
    return null;
  }
};

/** The store collection of this module. */
export const AUTH = "auth";

const sessionCell = cell<Session | null>(AUTH, "session", readSession());
const meCell = cell<MeResponse | null>(AUTH, "me", null);
const restoringCell = cell(AUTH, "restoring", false);

function setSession(s: Session | null): void {
  sessionCell.set(s);
  writeStore(SESSION_KEY, s ? JSON.stringify(s) : null);
}

// Follow another tab that resumes, acts as someone, or signs out.
if (typeof window !== "undefined") {
  window.addEventListener("storage", (e) => {
    if (e.key === SESSION_KEY) sessionCell.set(readSession());
  });
}

export const session = (): Session | null => sessionCell.get();
export const me = (): MeResponse | null => meCell.get();
export const restoring = (): boolean => restoringCell.get();
export const setRestoring = (v: boolean) => restoringCell.set(v);
export const storedSession = (): Session | null => readSession();
export const isAuthenticated = () => session() !== null;

/** Synchronous bearer read for the transport. */
export const currentToken = (): string | null => session()?.token ?? null;

export const signIn = (s: Session) => setSession(s);
export const setMe = (m: MeResponse | null) => meCell.set(m);

export const signOut = () => {
  setSession(null);
  meCell.set(null);
};

/** True while the bearer acts as another person. */
export const isActing = (): boolean => {
  const s = session();
  return !!s && s.via !== "self" && !!s.actingAs && s.actingAs.userId !== s.user.userId;
};

/** The person whose authority the bearer uses. */
export const effectivePerson = (): PersonSummary | null => {
  const s = session();
  if (!s) return null;
  return s.actingAs ?? s.user;
};

/**
 * Does the current bearer allow this scope? A self bearer allows all.
 * Scopes come from /me when known, else from the login response.
 */
export const hasScope = (scope: Scope): boolean => {
  const s = session();
  if (!s) return false;
  if (s.via === "self") return true;
  const scopes = me()?.scopes ?? s.scopes ?? [];
  if (scopes.includes(scope)) return true;
  // calendars.read includes freebusy.read.
  return scope === "freebusy.read" && scopes.includes("calendars.read");
};

export const isSelf = (): boolean => session()?.via === "self";
