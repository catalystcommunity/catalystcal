# Request: act-as scopes and reliable calendar operations for Longhouse

Date: 2026-11-05
Consumer: Longhouse
Dependency: catalystcal Go client v0.1.2

## Problem

Longhouse uses catalystcal as an optional calendar service. Longhouse needs
`calendars.read` and `events.write` for member operations. It does not need
`freebusy.read`, `events.respond`, or `calendars.manage`.

`AuthService.get-act-as-scope-set` accepts the grantee identity and language
preferences. It always returns a signed scope set with all five scopes.
Longhouse cannot remove entries from that set without breaking its signature.
The member can reduce the selection at consent, but Longhouse must not ask
for calendar management rights that it does not need.

## Required change

Add an optional `scopes: [* text]` field to `ScopeSetRequest` in CSIL.

- When `scopes` is absent, keep the current behavior for existing clients.
- When `scopes` is present, sign a set with exactly those scopes.
- Reject an empty list, an unknown scope, or a duplicate scope with 400.
- Use the existing scope order in the signed result.
- Keep the existing grantee, language, signature, and expiry checks.
- Do not let this option grant a scope that the audience does not support.

Longhouse will send:

```text
scopes = ["calendars.read", "events.write"]
```

The consent screen must show only those two scopes. The member can still
reduce the selection. Longhouse will require both scopes before it stores a
usable connection.

## Validation

Test the two-scope request through the HTTP CSIL carrier. Decode the returned
signed scope set and verify its signature with the regular-RP SDK. Check that
the set contains exactly the two requested entries. Test the absent, empty,
unknown, duplicate, and reordered scope lists. Keep a test for the old client
request without `scopes`.

Regenerate all clients and publish a Go client release. Longhouse currently
uses `github.com/catalystcommunity/catalystcal/clients/go@v0.1.2`, with no local
replacement. Please report the release version when it is available.

## Durable event creation

Longhouse stores an admin assignment locally. Only its assigned owner can
accept it. Longhouse records the accepted revision and a stable creation key
before it calls catalystcal with that owner's act-as bearer.

If catalystcal commits the event but its response is lost, Longhouse must
recover the result. A unique live UID alone is not sufficient. The event can
move, change, enter trash, or leave trash before Longhouse retries. Searching
by title is not reliable, and range reads cannot find an event that moved
outside the range.

Add an optional `idempotency_key` to the create-event request. Keep the old
request valid when the field is absent. Use a durable receipt with these
rules:

- Scope the key to the calendar and the effective creator identity.
- Bind it to the authenticated grantee application when the call uses act-as.
- Atomically store the receipt and create the event.
- A retry with the same key and same original content must identify the same
  event. It must not create another event.
- A retry with changed original content must return 409.
- Keep the receipt after event edits, moves, deletion, trash expiry, and
  restore. Do not reuse the key for a second event.
- Check current authorization on every retry and receipt lookup. A receipt
  must not give a removed member access to a calendar or event.
- Return enough information to distinguish a live event from a deleted
  result. Never restore or recreate an event as a side effect of a retry.

A separate lookup operation is acceptable if it has the same binding and
authorization checks. It must return the original event ID and its current
state. Please specify the final request and response types in your response
to this request, so Longhouse can use the released generated client.

Test two simultaneous creates with one key, a lost response followed by a
retry, changed content, another creator, another calendar, and another
grantee. Also test retries after a move, deletion, trash expiry, restore,
membership removal, and process restart.

The same recovery rule is needed when `update-event` splits a series with
`scope = this_and_future`. Longhouse must update its stored future references
to the successor series. If the split succeeds but its response is lost,
Longhouse needs the original and successor series IDs from a durable result.
Please support an idempotency key for this operation too, or provide an
equivalent split receipt. A retry must not split the series again. Return the
split boundary and identify how the original recurrence IDs map to the new
series when the edit changes its start time or recurrence rule.

## Stable occurrence lookup

Longhouse links and comments use a stable local reference. A published
occurrence maps to a catalystcal series ID and its original `recurrence_id`.
Moving that occurrence must not change its reference. `get-event` accepts
only an event ID. `list-events` finds occurrences by their current time, so
it cannot resolve a moved occurrence from its original timestamp alone.

Add `get-occurrence` with `event_id` and optional `recurrence_id`. Return the
same `Occurrence` shape as `list-events`.

- With a recurrence ID, resolve the original series occurrence and apply
  its override, even when the override moved outside the original range.
- Return the original recurrence ID and the current start and end times.
- Return 404 for a deleted, excluded, nonexistent, or inaccessible occurrence.
- With no recurrence ID, permit a single event. Require a recurrence ID for
  a recurring series, or document an explicit first-occurrence rule.
- Apply the same visibility and authorization checks as calendar reads.
- Resolve overrides without requiring the caller to know an override row ID.

Test a normal instance, a moved override, a move across dates and time zones,
an all-day instance, an excluded instance, deletion, restore, and access loss.
For a series split, document which series ID names the retained future
instances. Longhouse will update its references from the split result.

## Next Actions

Implement and release these three API changes in catalystcal. Then update
the Longhouse dependency. Keep the Longhouse consent flow closed if the
audience returns extra scopes. Do not retry an uncertain event creation
without the durable receipt contract.

## Response from catalystcal (2026-11-05)

Status: built in the catalystcal working tree. Not committed and not
released. The next release (expected `v0.1.3`, Go client tag
`clients/go/v0.1.3`) has these changes. catalystcal reports the version
when the release job tags it. Do not change the Longhouse dependency
before that tag.

### 1. Requested scopes

```cddl
ScopeSetRequest = {
    grantee_domain: text,
    grantee_user_id: text,
    grantee_application_id: text,
    ? languages: [* text],
    ? scopes: [* text]
}
```

- `scopes` absent: the set has all five scopes (old clients).
- `scopes` present: the set has exactly those scopes, in the server order
  (`calendars.read`, `freebusy.read`, `events.write`, `events.respond`,
  `calendars.manage`).
- An empty list, an unknown word, or a repeated word gives 400.
- Longhouse sends `scopes = ["calendars.read", "events.write"]`.

Go: `ScopeSetRequest.Scopes []string`. A nil slice is absent. An empty
non-nil slice is sent as an empty list and gives 400.

### 2. Durable event creation

The change is two new operations, not a field on `create-event`.
`create-event` takes a bare `Event` and returns an `Event`. It cannot tell
a deleted result from a live one, and a new field on `Event` would be on
every response. `create-event` is not changed.

```cddl
CreateReceiptState = "live" / "trashed" / "deleted" / "hidden"

CreateEventOnceRequest = {
    idempotency_key: text .size (1..200),
    event: Event
}

CreateReceiptRequest = {
    calendar_id: CalendarID,
    idempotency_key: text .size (1..200)
}

CreateEventReceipt = {
    calendar_id: CalendarID,      ; the calendar of the create
    idempotency_key: text,
    event_id: EventID,            ; never changes
    state: CreateReceiptState,
    created: bool,                ; true only for the call that made it
    created_at: Timestamp,
    ? event: Event,               ; state live
    ? trash_id: TrashID           ; state trashed, when the caller can restore
}

service EventService {
    create-event-once:  CreateEventOnceRequest -> CreateEventReceipt / ServiceError,
    get-create-receipt: CreateReceiptRequest   -> CreateEventReceipt / ServiceError
}
```

Go: `EventClient.CreateEventOnce` and `EventClient.GetCreateReceipt`.

Rules:

- Scope: calendar, effective creator, and bearer. For an act-as bearer
  the bearer part is the exact grantee
  (`<user_id>@<domain>/<application_id>`). It is not the grant id, so a
  renewed or new grant from the same application finds the same receipt.
  For a delegated bearer it is the delegate.
- The receipt and the event are written in one SQLite transaction. The
  receipt has a primary key on the scope and the key. Two creates at the
  same time make one event. The other call gets the receipt with
  `created` false.
- Same key and same content: no write, `created` false. The content is
  the `event` field as sent, without its receive-only fields. Same key
  and other content: 409.
- The receipt stays after edits, moves, delete, restore, and trash purge.
  A call never restores or makes an event again.
- `state`: `live` (the event is on a calendar and the caller can see it,
  `event` is set), `trashed` (in trash), `deleted` (the trash entry
  expired), `hidden` (the event exists, but the caller cannot see it now,
  for example after a move to a calendar without access).
- Each call checks access now. The caller must be able to add events to
  the receipt calendar. A member without access gets 404 (no access) or
  403 (read access only). The receipt gives no access.
- 404 from `get-create-receipt` when no receipt exists for the key. Then
  the create did not commit, and a retry with `create-event-once` is
  safe.

Longhouse use: store the key and the accepted revision first. Call
`create-event-once`. On a lost response, call it again with the same key
and the same event. Never retry with `create-event`.

### 3. Stable occurrence lookup

```cddl
GetOccurrenceRequest = {
    event_id: EventID,
    ? recurrence_id: Timestamp
}

service EventService {
    get-occurrence: GetOccurrenceRequest -> Occurrence / ServiceError
}
```

Go: `EventClient.GetOccurrence`.

- The answer is the `list-events` `Occurrence`. `recurrence_id` is the
  original start. `start` and `end` are the times now, with the override
  applied, also when the override moved the instance out of the original
  range or to another date.
- A single event: no `recurrence_id`. A `recurrence_id` on a single event
  gives 404.
- A series: `recurrence_id` is required (400 when absent). There is no
  first-occurrence rule.
- An override row id also works. The server uses its series and its
  `recurrence_id`. The caller does not need to know the override id.
- 404: excluded, cancelled, after the end of the series, deleted, a
  `recurrence_id` that is not an instance, or no access. The `list-events`
  visibility rules apply.
- All-day series: the date of `recurrence_id` counts. `start` and `end`
  are `00:00:00Z` of the dates, as in `list-events`.
- Series split: an `update-event` with scope `this_and_future` returns the
  new series. Its id names the instance and the later ones. The old
  series ends before the instance. After the split, (old `event_id`,
  later `recurrence_id`) gives 404. Update the references from the
  `update-event` result. If the instance is the first instance of the
  series, the update changes the series in place and the id stays.

### Tests

All through the HTTP CSIL carrier (`api/internal/httpapi`):

- `TestScopeSetRequestedScopes`: real signing, verified with the
  regular-RP SDK `VerifyScopeSet`. Absent, two scopes, reordered, empty,
  unknown, duplicate.
- `TestCreateEventOnce`: lost response and retry, changed content (409),
  another calendar, edit, move, delete (trashed, and no restore by the
  retry), restore, trash expiry and purge (deleted), process restart.
- `TestCreateEventOnceConcurrent`: eight parallel calls with one key.
- `TestCreateEventOnceScopeAndAccess`: another creator, a move to a
  calendar without access (hidden), membership removal (404), a second
  credential exchange by the same grantee, another grantee.
- `TestGetOccurrence`: single event, normal instance, a moved override,
  a move across dates, a Denver series across the change to daylight
  time, an override id, an excluded instance and restore, a series delete
  and restore, a split, an all-day series, and access loss.
- `store.TestCreateEventWithReceiptOnce`: a taken key writes no event, and
  a taken uid writes no receipt.

### Squizzlezig

- Act-as is on at `https://catalystcal.squizzlezig.com`. catalystcal's
  application identity is
  `01a18ed6-21fe-7803-b8dd-0ede08120c03@squizzlezig.com/catalystcal`,
  instance `catalystcal-squizzlezig-1`. A live scope set verified with
  the three enrolled keys.
- Application sign-in (`allowedApplications`) is off. Send the exact
  Longhouse squizzlezig application identity
  (`<user_id>@squizzlezig.com/<application_id>`) to turn it on. Until
  then, `get-application-identity` gives 503.
- Since 2026-11-06, squizzlezig runs the dev image
  `dev-20261105-214800-e89b5f8-dirty`, which has these changes. A live
  request with `scopes: ["calendars.read", "events.write"]` gave a
  signed set with only those two scopes. An unknown scope gave 400. The
  three new operations are live. Without a bearer they give 401.
- The server renews the key attestations at start and every 6 hours.

## Longhouse response (2026-11-07)

The scope filter, create receipt, and occurrence lookup contracts meet the
three main requests. The separate `create-event-once` operation is acceptable.

The released Go client is now `clients/go/v0.2.0`, not the expected v0.1.3.
Longhouse uses v0.2.0 without a local replacement. It sends the two requested
scopes, stores the exact accepted create request and its key, recovers the
receipt after a lost response, and resolves moved occurrence links with
`get-occurrence`. Recovery does not create or restore an event that was
deleted, moved, or put in trash.

### Remaining requirement: series splits

The response explains the result of a successful `update-event` split.
It does not cover the requested recovery when that response is lost.
The create receipt cannot recover an update or identify its successor series.

Please add an idempotent split operation or an equivalent durable receipt:

- Bind the key to the authenticated caller, grantee, and source series.
- Store the split and its receipt in one transaction.
- Return the original series ID, successor series ID, and split boundary.
- Let the same key and original request recover that result after a lost
  response, restart, later edit, deletion, or move. Do not split again.
- Reject reuse of the key with changed content.
- Specify how each existing future recurrence ID maps to the new series
  when the start time or rule changes. If no occurrence remains, identify
  that result so Longhouse can keep its link and comments without binding
  them to another occurrence.
- Apply current authorization checks on execution and recovery.

An ordinary split result is insufficient: after a lost response, the old
reference returns 404 and Longhouse has no reliable successor ID or mapping.
Longhouse currently rejects `this_and_future` edits before any remote call.

### Next Actions

1. catalystcal: supply the split recovery contract, tests, and released
   client version. Keep this request open for that requirement.
2. Longhouse: consume that contract and update stored occurrence references.
3. Deployment: enroll the Longhouse application, provide its exact identity
   for the catalystcal allow-list, and test consent and member operations
   against the live service.

## Response from catalystcal (2026-11-07): series splits

Added: `split-series-once` and `get-split-receipt`. Expected release: the
next minor version after `v0.2.0` (`clients/go/v0.3.0`), not tagged yet.
On squizzlezig only after the next deploy (see below).

```cddl
SplitSeriesOnceRequest = {
    idempotency_key: text .size (1..200),
    event: Event,               ; event.event_id = the series master
    recurrence_id: Timestamp,   ; the instance where the split starts
    ? map_recurrence_ids: [* Timestamp]   ; at most 500, not hashed
}

SplitReceiptRequest = {
    series_id: EventID,         ; event.event_id of the split request
    idempotency_key: text .size (1..200),
    ? map_recurrence_ids: [* Timestamp]
}

SplitMapResult = "original" / "successor" / "none"

SplitOccurrenceMapping = {
    recurrence_id: Timestamp,             ; as requested
    result: SplitMapResult,
    ? event_id: EventID,                  ; original and successor
    ? mapped_recurrence_id: Timestamp
}

SplitSeriesReceipt = {
    series_id: EventID,
    idempotency_key: text,
    successor_id: EventID,
    split_at: Timestamp,        ; the split boundary
    in_place: bool,
    created: bool,              ; true only for the call that split
    created_at: Timestamp,
    calendar_id: CalendarID,    ; the calendar of the split
    series_state: CreateReceiptState,
    ? series: Event,
    ? series_trash_id: TrashID,
    successor_state: CreateReceiptState,
    ? successor: Event,
    ? successor_trash_id: TrashID,
    mappings: [* SplitOccurrenceMapping]  ; request order
}

service EventService {
    split-series-once: SplitSeriesOnceRequest -> SplitSeriesReceipt / ServiceError,
    get-split-receipt: SplitReceiptRequest    -> SplitSeriesReceipt / ServiceError
}
```

Go: `EventClient.SplitSeriesOnce` and `EventClient.GetSplitReceipt`.

Each requirement:

- **Binding.** The key is scoped to the series id of the request, the
  effective user, and the bearer: the exact grantee
  (`<user_id>@<domain>/<application_id>`) for act-as, the delegate for a
  delegated bearer.
- **One transaction.** The series change, the override removal, the new
  series, and the receipt are written in one transaction. Eight parallel
  calls with one key make one split.
- **Result.** `series_id`, `successor_id`, and `split_at`. A
  `recurrence_id` at or before the first instance changes the series in
  place: `in_place` true, `successor_id` = `series_id`, `split_at` = the
  first instance.
- **Recovery.** The same key and the same `event` and `recurrence_id`
  return the receipt (`created` false) after a lost response, a restart,
  a later edit, a delete, a move, or a purge. It never splits again and
  never restores. `series_state` and `successor_state` are `live`,
  `trashed` (with a trash id when the caller can restore), `deleted`, or
  `hidden`.
- **Changed content.** A known key with another `event` or
  `recurrence_id` gives 409.
- **Mapping.** For each value in `map_recurrence_ids`, a recurrence id
  of the series before the split:
  - `original`: before the split. The same series and recurrence id.
  - `successor`: at or after the split. `mapped_recurrence_id` =
    R + (successor start − `split_at`), in the local wall-clock time of
    each series (UTC for all-day). A split across daylight time keeps the
    local time. An all-day successor maps to `00:00:00Z` of the date.
  - `none`: R was not an instance before the split (excluded, not on the
    rule, after the end), or the mapped time is not an instance of the
    successor (for example a weekly rule that becomes every two weeks, or
    a shorter COUNT). Keep the link and comments unbound.

  The receipt keeps the series before the split and the successor as the
  split wrote them. The mapping uses only these copies, so it does not
  change after later edits. Call `get-occurrence` on the mapped reference
  for the occurrence now.
- **Authorization.** On execution and on each recovery, the caller must
  be able to change the series on the calendar of the split: write
  access, or contribute access to a series that the caller created. Else
  404 or 403, with nothing shown.

Rules for the request:

- `event.event_id` must be a series master. A single event or an
  override id gives 400.
- `event.start`/`end` (or `start_date`/`end_date`) are the new times of
  the instance at `recurrence_id`. This is what the web app sends. If
  `event.recurrence` is absent, the successor keeps the rule and the
  later EXDATE and RDATE values.
- The overrides of instances at and after the split are removed, as
  with `update-event`. The mapping still gives the new recurrence id.
- Send the same `event` bytes on a retry. The server hashes the event
  without its receive-only fields, plus `recurrence_id`. Store the exact
  accepted request with the key, as for creates.

Longhouse use: store the key and the request before the call. Call
`split-series-once` with the stored occurrence references in
`map_recurrence_ids`. On a lost response, call it again with the same key
and request, or call `get-split-receipt`. Then rebind each reference from
the mappings.

Tests (`api/internal/httpapi/split_test.go`, `store.TestSplitWithReceiptOnce`):
lost response and retry, 409 for other content and another recurrence id,
the mapping (before, excluded, off-rule, after a shorter rule, after the
end), edit, move, delete, purge, restart, an in-place change, Denver
across daylight time, an all-day series, eight parallel calls, contribute
access, membership removal, and a taken key that rolls the split back.

### Next Actions

1. catalystcal: merge and release (expected `clients/go/v0.3.0`), then
   deploy to squizzlezig.
2. Longhouse: allow `this_and_future` edits through `split-series-once`
   and rebind stored references from the mappings.
3. Deployment: send the Longhouse squizzlezig application identity for
   `allowedApplications`. It is still the open item.
