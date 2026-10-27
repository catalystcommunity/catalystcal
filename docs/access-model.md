# Access model

This document gives the rules that decide who can do what in catalystcal. The code
must follow these rules. The tests must prove them.

## People

A person is a LinkKeys identity: a domain and a stable account id (the
subject). catalystcal makes a `users` row at the first sign-in. A share or a
delegation can name a person before that person signs in, because it stores
the domain and the subject, not the `users.id`.

Domains compare without case. The store writes domains in lower case.

A request can name a person by handle (`IdentityRef.handle`). The server
finds the handle only among people who signed in to this server. If the
server cannot find the handle, it returns 404 with the message "that person
has not signed in to this catalystcal server yet; use their account id".

## Three access paths

| Path | Who | Acts under the name of | Set by |
| --- | --- | --- | --- |
| Owner | The calendar owner | Owner | Calendar creation |
| Share | A grantee with a `ShareRole` on one calendar | The grantee | `ShareService.put-share` |
| Delegation | A delegate with scopes from a principal | The principal | `DelegationService.create-delegation` |

An attendee also sees an event that names them, on any calendar, with read
access to that one event only.

### Shares: "as themselves"

Roles, from low to high: `free_busy`, `read`, `write`, `manage`.

| Role | Can do |
| --- | --- |
| `free_busy` | See busy time only. Events come back with `busy_only = true` and no title, description, location, attendees, or reminders. |
| `read` | See event details. A `private` or `confidential` event shows as busy only. |
| `write` | `read`, plus create, change, move, and delete events. |
| `manage` | `write`, plus change the calendar settings, the feed, and the shares. Cannot delete the calendar. |

An event that a share grantee creates has `created_by` and `organizer` set to
the grantee.

### Delegations: "act as me"

A delegation has a principal, a delegate, a list of scopes, an optional list
of calendars, and an optional expiry.

| Scope | Lets the delegate |
| --- | --- |
| `calendars.read` | List the principal's calendars and see event details, including private events. |
| `freebusy.read` | See the principal's busy time. `calendars.read` includes this. |
| `events.write` | Create, change, move, and delete events on the principal's calendars, as the principal. |
| `events.respond` | Answer invitations sent to the principal, as the principal. |
| `calendars.manage` | Change calendar settings, feeds, and shares of the principal's calendars. |

A delegation with calendar ids applies only to those calendars. A
delegation without calendar ids applies to every calendar the principal
owns, including later ones. Invitations to the principal are not on a
calendar: `events.respond` with a calendar list still applies to them.

How a delegate uses a delegation:

1. The delegate signs in with their own account. The bearer is a self
   bearer.
2. The delegate calls `AuthService.assume-principal`. The server checks
   that a live delegation exists and mints a delegated bearer:
   `via = delegation`, `principal`, `delegation id`, `scopes`. The bearer
   lives for at most one hour and never past the delegation expiry.
3. Each call with the delegated bearer runs as the principal. The
   dispatcher checks the delegation in the store before each call. A
   revoked or expired delegation fails the call with "unauthenticated" at
   once.
4. `AuthService.release-principal` mints a self bearer for the delegate
   again.

What a delegate can never do with a delegated bearer:

- Call `DelegationService` (any op). A delegate cannot grant, extend, or
  see delegations of the principal.
- Delete a calendar of the principal.
- Call `assume-principal` (no chains).
- Call `UserService.update-profile`.

The audit log records each mutation and each delegated call: the actor (the
delegate), the principal, `via`, the op, the target, and the outcome. The
principal can read it with `AuditService.list-audit`. An event changed by a
delegate has `updated_by` = the principal and `updated_via = delegation`.
The audit log keeps the real actor.

### LinkKeys act-as grants (applications)

A LinkKeys act-as grant lets an application (the grantee) act as a user at
catalystcal (the audience). It uses the same scope words.

1. The grantee asks catalystcal for a signed scope set:
   `AuthService.get-act-as-scope-set`. catalystcal signs it with its application
   keys. This op works only when the server has application keys
   (`CATALYSTCAL_ACT_AS_APP_KEYS_FILE`).
2. The user approves the grant at their home domain.
3. The grantee calls `AuthService.exchange-act-as-credential` with an
   `ActAsCredential`. The presentation's request digest is SHA-256 of
   `catalystcal/AuthService/exchange-act-as-credential`. catalystcal verifies the
   credential with the LinkKeys regular-RP Go SDK, records the presentation
   nonce, and mints a bearer: `via = act_as_grant`, the user as principal
   and as actor, the grant id, the grantee application, and only the
   approved scopes. The bearer never outlives the grant.
4. The dispatcher re-checks the grant id against the known revocations.

## Effective access, in one function

For a calendar `c` and a caller identity `id`, effective access is:

1. `id.EffectiveUserID() == c.owner_id`:
   - Self bearer: `owner`.
   - Delegated bearer and `c` in the delegation's calendars: map the scopes.
     `calendars.manage` + `events.write` → `owner` minus delete-calendar;
     `events.write` → `write`; `calendars.read` → `read` (private events
     visible); `freebusy.read` → `free_busy`; else `none`.
2. Else a share for the actor's identity on `c` → that role. A delegated
   bearer does not use the principal's shares on other people's calendars
   unless the scopes allow it: `calendars.read` for read, `events.write`
   for write.
3. Else `none`.

Additional rules:

- `calendars.manage` alone gives no access. It adds settings, feeds, and
  sharing to the access that the other scopes give.
- A delegation that is limited to a list of calendars never gives access
  to calendars that other people shared with the principal.
- A delegated bearer sees the principal's invitations only with
  `calendars.read` or `events.respond`.
- `ShareService.put-share` creates the share or changes its role.
- The grantee of a share can delete that share.
- Search never returns an event that the caller can see only as busy.

The services call one shared function for this (`api/internal/access`).
They do not repeat the rules.

## Errors

| Case | Code |
| --- | --- |
| No bearer, bad bearer, revoked delegation | transport status unauthenticated |
| Calendar or event does not exist, or the caller has `none` access | 404 (do not reveal existence) |
| The caller can see it but cannot do this | 403 |
| Bad input | 400 |
| Conflict, for example a duplicate share | 409 |
