/**
 * Parse what a person types to name somebody into an IdentityRef.
 *
 *   alice@example.com             -> { domain: "example.com", handle: "alice" }
 *   01J9Z...(26 chars)@example.com -> { domain, userId }   (a ULID or UUID)
 *   id:abc123@example.com          -> { domain, userId: "abc123" }
 *   alice                          -> { domain: defaultDomain, handle: "alice" }
 *
 * The server finds a handle only among people who signed in to this catalystcal
 * server; an account id works for anybody.
 */
import type { IdentityRef } from "@catalystcal/client";

const ULID = /^[0-9A-HJKMNP-TV-Z]{26}$/i;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const DOMAIN = /^(?=.{1,253}$)[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*(?::\d{1,5})?$/i;
const LOCAL = /^[a-z0-9][a-z0-9._+-]*$/i;

export function looksLikeAccountId(s: string): boolean {
  return ULID.test(s) || UUID.test(s);
}

export function parseIdentity(input: string, defaultDomain?: string): IdentityRef | null {
  let s = input.trim();
  if (!s) return null;
  if (s.startsWith("@")) s = s.slice(1);
  let forceId = false;
  if (/^id:/i.test(s)) {
    forceId = true;
    s = s.slice(3);
  }
  const at = s.lastIndexOf("@");
  let local = at >= 0 ? s.slice(0, at) : s;
  let domain = at >= 0 ? s.slice(at + 1) : (defaultDomain ?? "");
  domain = domain.toLowerCase();
  if (!local || !domain || !DOMAIN.test(domain)) return null;
  if (forceId || looksLikeAccountId(local)) {
    if (!/^[A-Za-z0-9._:-]+$/.test(local)) return null;
    return { domain, userId: local };
  }
  if (!LOCAL.test(local)) return null;
  return { domain, handle: local };
}

export function formatIdentity(ref: IdentityRef): string {
  if (ref.handle) return `${ref.handle}@${ref.domain}`;
  if (ref.userId) return `${looksLikeAccountId(ref.userId) ? "" : "id:"}${ref.userId}@${ref.domain}`;
  return ref.domain;
}

export function sameIdentity(a: IdentityRef | undefined, b: IdentityRef | undefined): boolean {
  if (!a || !b) return false;
  if (a.domain.toLowerCase() !== b.domain.toLowerCase()) return false;
  if (a.userId && b.userId) return a.userId === b.userId;
  if (a.handle && b.handle) return a.handle.toLowerCase() === b.handle.toLowerCase();
  return false;
}

export function isEmail(s: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.trim());
}
