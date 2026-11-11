# catalystcal: notes for agents

catalystcal is a calendar service: calendars, events, shares, delegations
(act-as), and iCalendar import, export, feeds, and subscriptions. It
replaces Apple, Google, or Outlook calendars and works with them through
iCalendar.

**Name.** Use exactly `catalystcal`, `CatalystCal`, and `CATALYSTCAL_` in
code, files, and docs. Do not use other spellings or abbreviations.

## Architecture

- **csil/catalystcal.csil** is the source of truth for the API. It drives the Go
  server code (`api/internal/csil`) and every client (`clients/<lang>/`).
  `csil/regenerate.sh` pins csilgen (`CSILGEN_PIN`, now 0.2.10) and stops
  when a different csilgen is on PATH.
- **api/** is the Go module `github.com/catalystcommunity/catalystcal/api`.
  `main.go` holds `var Version = "dev"`. Builds set it with
  `-ldflags "-X main.Version=<version>"`. Subcommands: `serve` (default),
  `migrate`, `app-keys` (make, enroll, and renew the act-as application
  keys), `version`. `serve` renews the key attestations every 6 hours.
  - Port 7080 (`CATALYSTCAL_API_PORT`): `POST /api/csil/v1/rpc` (also
    `/csil/v1/rpc`), `GET /api/v1/auth/start`, `GET /api/health`,
    `GET /ics/{token}` and `GET /api/ics/{token}`.
  - Port 7081 (`CATALYSTCAL_OPS_PORT`): `/healthz` (liveness) and `/readyz`
    (readiness). Keep the ops port off the public routes.
  - `/ics/*` must be public. Calendar apps fetch feeds with no session.
- **coredb/** is a separate Go module with embedded goose migrations for
  SQLite. `api/go.mod` has `replace github.com/catalystcommunity/catalystcal/coredb
  => ../coredb`, so a build needs both directories.
- **webapp/** is the SPA: Zest web components (`@catalystcommunity/*` from
  npm), TypeScript, and Vite. It has no application framework. Do not add
  one. It imports `@catalystcal/client`, an alias to
  `../clients/typescript`. It reads `../version/VERSION.txt` at build
  time. The SPA handles `/auth/callback`. Vite dev runs on 5180 and proxies
  `/api` and `/ics` to 7080.
  - Each part of the UI is a custom element that extends `CalElement`
    (`src/ui/element.ts`, a Zest `BaseComponent` in the light DOM).
    `build()` makes the elements one time. `render()` changes only the
    parts that changed. `connected()` adds listeners with `own` and
    `listen`. Make elements with `h()` from `src/ui/dom.ts`. Use
    `syncKeyed` for a list with controls in its rows, so that the focus
    stays.
  - App state is in one `BasicDataService` (`src/state/store.ts`). Each
    `src/stores/` module owns one collection and gives typed functions.
    An element lists the collections that it shows in `static watches`.
  - Routes use the Zest `SimpleRouter` (`src/app/router.ts`). The route is
    in the store. `navigate()` changes it.
  - Use the Zest controls, containers, menus, `calendar-view`, and
    `date-time-picker`. `src/ui/zest.ts` defines their tags at boot.
    Use the TypeScript types that each Zest package gives. Do not
    declare them again in this repo.
  - Zest classes on the inner elements start with `zest-`. Do not give
    an app class the `zest-` prefix.
  - A `push-button` in the `buttons` slot of `dialog-window` closes the
    dialog. `openModal` adds `data-keep-open`, so that a Save handler
    can close the dialog after the save. An event state on the calendar
    is an item tag (`itemTags`).
  - Make a component specific to catalystcal in this repo, as a
    `CalElement`. Change a general component in `../zest`.
  - After an `await`, call `this.isActive()` before you change the
    element. Ignore a response that is older than the newest request.
- **Dockerfile.api**: distroless static image, nonroot (65532), database on
  the `/data` volume.
- **Dockerfile.web**: Caddy serves the SPA and proxies `/api/*` and `/ics/*`
  to `CATALYSTCAL_API_UPSTREAM` (default `api:7080`). `webapp/Caddyfile` holds the
  cache and security headers.
- **helm_chart/**: one API pod (Recreate, PVC), a web Deployment that can
  scale, an HTTPRoute or an Ingress, an optional LinkKeys RP, an optional
  act-as keys mount, and an optional backup CronJob.
- **.reactorcide/**: CI. See "CI" below.

## Conventions

- **CSIL first.** To change the API, edit `csil/catalystcal.csil`, then run
  `./tools gen`. Never edit generated files (`*.gen.*`, `api/internal/csil/`,
  `clients/`). Commit new generated files too: `./tools gen-check` and CI
  fail on changed AND on untracked generated files.
- **Client overlays.** Hand-written client code lives in
  `clients/overlays/<lang>/`. `clients/regenerate.sh` copies it over the
  generated package. Edit the overlay, never the copy. The Go overlay has
  the HTTPS carrier, application presentation signing (`AppSigner`), and
  bearer sessions (`AppSession`, `ActAsSession`). Its
  `internal/transport` is a copy of `api/internal/transport`, so the Go
  client has no dependencies and one tag (`clients/go/vX.Y.Z`, made by the
  release job).
- **Principals and groups.** A principal is a person or an application
  principal (`users.application_id` set). Compare identities by domain,
  account id, AND application id. A share grantee is an identity or a
  group. Access through groups is read at each call: never cache it in a
  bearer.
- **Application presentations** are a catalystcal type, not a LinkKeys type.
  The tag `catalystcal-application-presentation-v1` and the digest input
  are in `api/internal/apppresent` and in the Go overlay. Change both, and
  both test vectors, together.
- **Trash is a snapshot table.** A delete removes the rows from `events`
  and writes an `event_trash` entry with a JSON snapshot
  (`store.TrashEvent`). Do not add a `deleted_at` column to `events`: then
  every read path must filter it. A new delete path must go through
  `TrashEvent`. `ReplaceCalendarEvents` (subscription sync) does not use
  trash.
- **Attributed import.** `import-event` is the one path that sets
  `created_by` to someone other than the caller. It can make a user row
  for a person who did not sign in (`store.EnsureUser`, no
  `last_seen_at`).
- **Create and split receipts are permanent.**
  `event_create_receipts.event_id` and `event_split_receipts.series_id` /
  `successor_id` have no foreign key. A delete, a move, or a purge must
  not remove a receipt. Only a calendar or user delete removes it
  (cascade). A split receipt keeps the series before the split and the
  successor as CBOR. The mapping uses only these copies. A
  `this_and_future` change goes through `planSplit`.
- **csilgen lint warnings are expected.** The house CSIL style causes them.
  Only errors fail `./tools lint`.
- **SQLite only.** The driver is `modernc.org/sqlite` (pure Go), so
  `CGO_ENABLED=0` builds work. Do not add PostgreSQL or a CGO driver.
- **One writer.** SQLite allows one writing process. Run one API process for
  each database file. The Helm chart has one API replica, `strategy:
  Recreate`, and no API autoscaler. Do not change this.
- **Time text layout.** An instant is text in ONE fixed layout: UTC, whole
  seconds, `Z` suffix (`2026-10-15T17:00:00Z`, Go layout
  `2006-01-02T15:04:05Z`). The database (TEXT columns) and CSIL use it. Text
  order is then time order, so `ORDER BY` and range checks work on the
  text. Do not write other layouts (offsets, fractions). An IANA time zone
  name (`America/Denver`) is a separate field. All-day events use dates, not
  instants.
- **One place for authorization.** The effective-access function decides
  what a caller can do on a calendar. Services call it. Services do not
  repeat the rules. [docs/access-model.md](docs/access-model.md) gives the
  rules.
- **Errors do not leak existence.** A missing calendar and a calendar with
  no access both give 404.
- **Environment variables** start with `CATALYSTCAL_`. `.env.dev.example` and
  `docs/deploy.md` list them.
- **Secrets.** Never put a secret in a commit, a values file, a script, or
  shell history. Use a Kubernetes Secret (`secrets.existingSecret`,
  `actAs.existingSecret`) or the Reactorcide secret store.

## Shared worktrees

More than one agent can work in this repo at the same time.

- Never run `git stash`, `git checkout -- <path>`, `git reset`, or
  `git clean`. These commands destroy the work of other agents.
- Edit only the files that your task owns.
- Do not `git add`, `git commit`, or `git push` unless the user asks.
- A build can fail because another agent is in the middle of a change. Do
  not fix their code. Report it.

## Commands

```sh
./tools dev / dev-down     # local API + Vite, SQLite at .data/catalystcal.db
./tools gen / gen-check    # regenerate from csil/; check for drift
./tools test               # test-api + test-web
./tools lint               # gofmt -l, go vet, csilgen lint
./tools helm-lint          # helm lint + helm template
./tools build-images [tag] # docker build both images
./tools compose-up / compose-down
```

Activate the shared toolchains first when you build a client in another
language:

```sh
source "${CATALYST_TOOLS:-$HOME/.local/catalyst-tools}/env.sh"
```

`./tools` does this for you when the file exists.

## CI (Reactorcide)

- `workflows/pr.yaml` (`id: catalystcal-pr`): conventional commits, CSIL
  gen-check, API tests, web tests, Helm lint, and a no-push build of both
  images.
- `workflows/release-tag.yaml` (`id: catalystcal-release-tag`): on a merge to
  main, semver-tags pushes `vX.Y.Z`. The job writes the version to
  `version/VERSION.txt` and `helm_chart/Chart.yaml`, pushes the bump, and
  attaches the packaged chart to a GitHub release.
- `workflows/release.yaml` (`id: catalystcal-release`): on a `v*` tag, build and
  push `containers.catalystsquad.com/public/catalystcal/{api,web}:<version>`.
- Job bodies are shell scripts in `.reactorcide/jobs/scripts/`. Jobs run
  them with `runnerlib run --job-command 'bash /job/ci/...'`. Pull request
  jobs run the scripts from the trusted base branch, not from the PR.
- Do not change a workflow `id`. The coordinator policy names it.
- Never commit a CI policy file. Policy is coordinator state.
- The csilgen pin is read from `csil/regenerate.sh`. CI installs that exact
  release and checks each asset against its GitHub SHA-256 digest.
