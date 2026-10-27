# catalystcal

catalystcal is a calendar service. It can replace the Apple, Google, or Outlook
calendar for a person, a family, or a team. It works with those calendars
through iCalendar (RFC 5545):

- Import and export `.ics` files.
- Publish a calendar as a feed (`/ics/<token>`). Apple Calendar, Google
  Calendar, and Outlook can subscribe to the feed URL.
- Subscribe to external calendar feeds. catalystcal fetches them on a schedule.

The API is CSIL-RPC (CBOR over HTTP, one endpoint). `csil/catalystcal.csil` is the
contract. Generated clients for 14 languages are in `clients/`. Sign-in uses
[LinkKeys](https://github.com/catalystcommunity/linkkeys).

## Shares and delegations

- A **share** gives a person a role on one calendar: `free_busy`, `read`,
  `write`, or `manage`. The person acts as themselves.
- A **delegation** lets a delegate act as you ("act as me") with scopes
  that you choose, on all or some of your calendars.
- An application can also act as you through a LinkKeys act-as grant.
- The audit log records the real actor for each delegated change.
- [docs/access-model.md](docs/access-model.md) gives the full rules.

## Quick start: local development

You need Go 1.26, Node 24, and npm. You do not need a database server: catalystcal
uses SQLite.

```sh
cp .env.dev.example .env.dev
./tools dev
```

- Open http://localhost:5180.
- Use the dev sign-in page. It is available because `.env.dev` sets
  `CATALYSTCAL_ENV=dev` and `CATALYSTCAL_DEV_AUTH_ENABLED=true`.
- The database is `.data/catalystcal.db`. Logs are in `logs/`.
- Press Ctrl-C to stop the log tail. The processes continue to run.
- Run `./tools dev-down` to stop the processes.

## Quick start: Docker Compose

```sh
docker compose up --build
```

Open http://localhost:8080. This setup is for a local try-out only. It
turns on the dev sign-in page. Do not expose it to a network.
[docs/deploy.md](docs/deploy.md) tells how to run catalystcal in production.

## Layout

| Path | Contents |
| --- | --- |
| `csil/` | `catalystcal.csil` (the API contract) and `regenerate.sh` |
| `api/` | Go API server (`serve`, `migrate`, `version`) |
| `api/internal/csil/` | Generated Go server code. Do not edit. |
| `coredb/` | SQLite migrations (goose), a separate Go module |
| `clients/<lang>/` | Generated clients. Do not edit. |
| `webapp/` | SolidJS + Vite web UI. It uses `clients/typescript`. |
| `helm_chart/` | Helm chart |
| `deploy/` | Example Helm values |
| `.reactorcide/` | CI workflows, jobs, and scripts |
| `docs/` | Access model, development, and deployment guides |
| `version/VERSION.txt` | The release version |

## Tests and checks

```sh
./tools test        # API and web UI tests
./tools test-api    # go vet + go test (api/ and coredb/)
./tools test-web    # typecheck, vitest, and build (webapp/)
./tools lint        # gofmt, go vet, csilgen lint
./tools gen-check   # regenerate from csil/ and fail on a difference
./tools helm-lint   # helm lint + helm template
```

Run `./tools help` for all commands. [docs/development.md](docs/development.md)
has more information.
