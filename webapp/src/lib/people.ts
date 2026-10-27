import type { IdentityRef, PersonSummary } from "@catalystcal/client";

/** The name to show for a person. */
export function personName(p: PersonSummary | undefined | null): string {
  if (!p) return "Someone";
  if (p.displayName) return p.displayName;
  if (p.handle) return `${p.handle}@${p.domain}`;
  return `${p.subject || p.userId}@${p.domain}`;
}

/** A short handle-ish address for a person. */
export function personAddress(p: PersonSummary | undefined | null): string {
  if (!p) return "";
  return `${p.handle ?? p.subject ?? p.userId}@${p.domain}`;
}

export function identityLabel(id: IdentityRef | undefined, person?: PersonSummary): string {
  if (person) return personName(person);
  if (!id) return "Unknown person";
  return `${id.handle ?? id.userId ?? "?"}@${id.domain}`;
}

export function initials(name: string): string {
  const parts = name.replace(/@.*/, "").split(/[\s._-]+/).filter(Boolean);
  const s = (parts[0]?.[0] ?? "?") + (parts.length > 1 ? parts[parts.length - 1][0] : "");
  return s.toUpperCase();
}

export function samePerson(a: PersonSummary | undefined | null, b: PersonSummary | undefined | null): boolean {
  return !!a && !!b && a.userId === b.userId;
}
