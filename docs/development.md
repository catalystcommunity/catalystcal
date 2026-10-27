# Development

This guide tells how to build, run, and test catalystcal on a workstation.

## Tools

You need:

- Go 1.26
- Node 24 and npm
- git

You need these tools only for some tasks:

- csilgen 0.2.9 (`./tools gen`, `./tools gen-check`, `./tools lint`)
- Docker with the compose plugin (`./tools build-images`, `./tools compose-up`)
- helm 3 (`./tools helm-lint`)

Run `./tools check-deps` to find missing tools.

`./tools` loads the shared Catalyst toolchains when
`${CATALYST_TOOLS:-$HOME/.local/catalyst-tools}/env.sh` exists. To use your own
copy of a tool, put its key in `CATALYST_TOOLS_SKIP` (for example
`export CATALYST_TOOLS_SKIP="go"`).

## Run catalystcal locally

1. Copy the example settings:

   ```sh
   cp .env.dev.example .env.dev
   ```

2. Start the API and the web UI:

   ```sh
   ./tools dev
   ```

3. Open http://localhost:5180.
4. Sign in on the dev sign-in page.

`./tools dev` does these steps:

- Reads `.env.dev`.
- Builds the API to `logs/catalystcal-api`. A compile error stops the command.
- Runs `npm ci` in `webapp/` when `webapp/node_modules` does not exist.
- Starts the API from the repo root. The database is `.data/catalystcal.db`.
- Starts Vite on port 5180. Vite sends `/api` and `/ics` to the API on
  port 7080.
- Writes the logs to `logs/api.log` and `logs/vite.log`, and the process
  IDs to `logs/*.pid`.
- Follows the logs. Ctrl-C stops the log tail only.

Use `./tools dev --no-tail` to start the processes and return at once.

To stop the processes:

```sh
./tools dev-down
```

`dev-down` keeps `.data/`. To start with an empty database, stop catalystcal and
remove `.data/catalystcal.db`, `.data/catalystcal.db-wal`, and `.data/catalystcal.db-shm`.

To run one part in the foreground:

```sh
./tools run-api    # the API only
./tools run-web    # Vite only
```

### Ports

| Port | Use |
| --- | --- |
| 5180 | Vite dev server (the SPA) |
| 7080 | API (`CATALYSTCAL_API_PORT`) |
| 7081 | Ops: `/healthz`, `/readyz` (`CATALYSTCAL_OPS_PORT`) |
| 8080 | Web container (Docker Compose) |

### Dev sign-in

The dev sign-in page works only when both conditions are true:

- `CATALYSTCAL_ENV` is `dev` or `nonprod`.
- `CATALYSTCAL_DEV_AUTH_ENABLED` is `true`.

Never turn on dev sign-in for an instance that other people can reach.

To test the real LinkKeys sign-in, set the `CATALYSTCAL_LINKKEYS_*` values in
`.env.dev`. [deploy.md](deploy.md) tells how to set up a LinkKeys relying
party.

## Change the API

1. Edit `csil/catalystcal.csil`.
2. Regenerate the server code and all clients:

   ```sh
   ./tools gen
   ```

3. Implement the change in `api/` and `webapp/`.
4. Commit the generated files with your change, including new files.

Do not edit generated files. They are `api/internal/csil/`, `clients/`,
and each `*.gen.*` file. The next `./tools gen` replaces them.

`csil/regenerate.sh` requires csilgen 0.2.9 exactly. A different version
can change the generated code without a schema change. If you have more than
one csilgen version, use an isolated install: put the 0.2.9 CLI first on
`PATH` and its generators in `$HOME/.csilgen/generators` of a separate
`HOME`. `.reactorcide/jobs/scripts/lib.sh` (`install_csilgen`) shows how.

## Database migrations

Migrations are SQL files in `coredb/migrations/`, embedded in the
`coredb` module with goose. To add a change, add a new numbered file. Do not
change a migration that a release already contains.

Run the migrations without the server (from the repo root):

```sh
./tools build
CATALYSTCAL_DB_PATH=./.data/catalystcal.db api/catalystcal migrate
```

## Tests and checks

| Command | What it does |
| --- | --- |
| `./tools test` | `test-api`, then `test-web` |
| `./tools test-api` | `go vet` and `go test` in `coredb/` and `api/` |
| `./tools test-web` | `npm run typecheck`, `npm test`, `npm run build` in `webapp/` |
| `./tools lint` | `gofmt -l`, `go vet`, `csilgen lint` |
| `./tools gen-check` | Regenerate, then fail when generated files change or new files appear |
| `./tools helm-lint` | `helm lint` and `helm template` with three sets of values |

`csilgen lint` shows many warnings for the house CSIL style. They are
expected. Only errors fail the command.

## Container images

```sh
./tools build-images          # tag = version/VERSION.txt
./tools build-images 0.1.0-me # another tag
```

The images are `catalystcal/api:<tag>` and `catalystcal/web:<tag>`. Set
`CATALYSTCAL_IMAGE_PREFIX` to use another name prefix.

Try the images with Docker Compose:

```sh
./tools compose-up     # http://localhost:8080
./tools compose-down   # keeps the catalystcal_data volume
```

## Versions

`version/VERSION.txt` holds the release version. CI changes it. Do not
change it by hand. A local build reports `<version>-dev`.
`./tools version` prints the version.

## CI

The CI runs on Reactorcide. The PR workflow runs these checks:

- Conventional commit subjects (`type(scope): description`)
- CSIL gen-check with the pinned csilgen
- API tests, web tests, and Helm lint
- A build of both images, with no push

Run the same checks locally before you push:

```sh
./tools lint && ./tools gen-check && ./tools test && ./tools helm-lint
```

Commit subjects must use one of these types: `feat`, `fix`, `docs`,
`style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`. The
release job uses them to calculate the next version: `feat` is a minor
release, `fix` and `perf` are a patch release, and `!` or `BREAKING CHANGE`
is a major release.
