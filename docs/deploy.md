# Deploy catalystcal

This guide tells how to run catalystcal for real users: with Docker Compose or
with the Helm chart. It also tells how to set up LinkKeys sign-in, act-as
application keys, backups, and upgrades.

## Parts

| Part | Image | Port | State |
| --- | --- | --- | --- |
| API | `containers.catalystsquad.com/public/catalystcal/api:<version>` | 7080 (API), 7081 (ops) | SQLite file on `/data` |
| Web | `containers.catalystsquad.com/public/catalystcal/web:<version>` | 8080 | None |

The web container (Caddy) serves the SPA. It also sends `/api/*` and
`/ics/*` to the API (`CATALYSTCAL_API_UPSTREAM`, default `api:7080`).

### Public paths

| Path | Goes to | Note |
| --- | --- | --- |
| `/api/*` | API | CSIL-RPC (`/api/csil/v1/rpc`), sign-in start, health |
| `/ics/*` | API | iCalendar feeds. Must be public: Apple, Google, and Outlook fetch them with no session. |
| everything else | Web | The SPA, including `/auth/callback` |

Do not make the ops port (7081) public. It is for probes only.

### One writer

The API keeps all data in one SQLite file. SQLite allows one writing
process. Thus:

- Run exactly ONE API process for each database file.
- Do not put the database on a network file system (NFS, SMB). SQLite
  locks and WAL shared memory do not work there. Use a local disk or a
  block volume (ReadWriteOnce).
- An upgrade stops the old API before the new API starts. Expect a short
  API outage on each upgrade.
- The web tier has no state. You can run more than one web container.

## Settings

The API reads `CATALYSTCAL_*` environment variables.

| Variable | Secret | Use |
| --- | --- | --- |
| `CATALYSTCAL_ENV` | no | `dev`, `nonprod`, or `prod` |
| `CATALYSTCAL_API_PORT` | no | API port (7080) |
| `CATALYSTCAL_OPS_PORT` | no | Ops port (7081): `/healthz`, `/readyz` |
| `CATALYSTCAL_DB_PATH` | no | SQLite file. Use `/data/catalystcal.db` in a container. |
| `CATALYSTCAL_JWT_SECRET` | **yes** | Token signing key. 32 bytes or more in prod. |
| `CATALYSTCAL_PUBLIC_URL` | no | The URL that people open, for example `https://catalystcal.example.com` |
| `CATALYSTCAL_CALLBACK_URL` | no | The SPA sign-in callback: `<public URL>/auth/callback` |
| `CATALYSTCAL_DEV_AUTH_ENABLED` | no | Dev sign-in page. Works only with `CATALYSTCAL_ENV` `dev` or `nonprod`. Keep `false` in prod. |
| `CATALYSTCAL_ALLOWED_DOMAINS` | no | Comma-separated LinkKeys domains that can sign in |
| `CATALYSTCAL_ALLOWED_USERS` | no | Comma-separated LinkKeys accounts that can sign in |
| `CATALYSTCAL_BEARER_TTL_SECONDS` | no | Bearer token lifetime |
| `CATALYSTCAL_SESSION_TTL_HOURS` | no | Browser session lifetime |
| `CATALYSTCAL_LINKKEYS_IDP_DOMAIN` | no | The LinkKeys identity domain |
| `CATALYSTCAL_LINKKEYS_IDP_URL` | no | The LinkKeys IDP base URL |
| `CATALYSTCAL_LINKKEYS_RP_DOMAIN` | no | This instance's relying-party domain |
| `CATALYSTCAL_LINKKEYS_PKI_API_KEY` | **yes** | RP API key |
| `CATALYSTCAL_LINKKEYS_TCP_ADDR` | no | RP CSIL-RPC/TCP `host:port`. Empty: DNS discovery. |
| `CATALYSTCAL_LINKKEYS_TCP_FINGERPRINTS` | no | Pinned SHA-256 certificate fingerprints. Empty: DNS discovery. |
| `CATALYSTCAL_LINKKEYS_TCP_ALLOW_INSECURE` | no | Skip fingerprint pinning. Dev only. |
| `CATALYSTCAL_SUBSCRIPTION_REFRESH_MINUTES` | no | How often catalystcal fetches subscribed calendars |
| `CATALYSTCAL_SUBSCRIPTION_ALLOW_PRIVATE` | no | Let subscriptions fetch private network addresses. Keep `false` on a shared instance. |
| `CATALYSTCAL_ACT_AS_APP_KEYS_FILE` | file is **secret** | Path to the act-as application keys JSON file |
| `CATALYSTCAL_ACT_AS_MAX_PRESENTATION_AGE_SECONDS` | no | The oldest act-as presentation that catalystcal accepts |

For a production instance, set at least:

- `CATALYSTCAL_ENV=prod`
- `CATALYSTCAL_DEV_AUTH_ENABLED=false`
- `CATALYSTCAL_JWT_SECRET` (make it with `openssl rand -base64 48`)
- `CATALYSTCAL_PUBLIC_URL` and `CATALYSTCAL_CALLBACK_URL`
- `CATALYSTCAL_ALLOWED_DOMAINS` or `CATALYSTCAL_ALLOWED_USERS`
- The `CATALYSTCAL_LINKKEYS_*` values for your IDP and RP

## Docker Compose

`docker-compose.yml` is for a local try-out. It turns on the dev sign-in
page and binds the ports to 127.0.0.1 only.

```sh
docker compose up --build
```

Open http://localhost:8080.

To use Compose for a small real instance:

1. Make a `.env` file next to `docker-compose.yml`. Set the production
   values from the list above. Compose reads `.env` automatically.
2. Put a TLS proxy in front of port 8080. The public URL must use HTTPS.
3. Start catalystcal: `docker compose up -d --build`.

The database is in the `catalystcal_data` volume. `docker compose down` keeps it.
`docker compose down -v` deletes it.

## Helm

The chart is in `helm_chart/`. `deploy/values-example.yaml` is a starting
point. It holds no secrets.

### Install

1. Make a namespace and the API Secret:

   ```sh
   kubectl create namespace catalystcal
   kubectl -n catalystcal create secret generic catalystcal-secrets \
     --from-literal=CATALYSTCAL_JWT_SECRET="$(openssl rand -base64 48)" \
     --from-literal=CATALYSTCAL_LINKKEYS_PKI_API_KEY="<RP API key>"
   ```

2. Copy `deploy/values-example.yaml`. Change the host names, the domains,
   and the Gateway or Ingress settings.
3. Install:

   ```sh
   helm upgrade --install catalystcal ./helm_chart -n catalystcal -f my-values.yaml
   ```

If you do not set `secrets.existingSecret`, the chart makes a Secret. It
makes a random `CATALYSTCAL_JWT_SECRET` on the first install and keeps it on each
upgrade. The RP API key then comes from `linkkeysRp.apiKey`.

### What the chart makes

- API Deployment: 1 replica, `strategy: Recreate`, a ReadWriteOnce
  PersistentVolumeClaim on `/data`, a read-only root file system, `/tmp`
  as an emptyDir, user 65532. Liveness is `/healthz` and readiness is
  `/readyz` on the ops port.
- The data PVC has `helm.sh/resource-policy: keep`. `helm uninstall` does
  not delete it.
- Web Deployment and an optional HorizontalPodAutoscaler
  (`autoscaling.web`). There is no API autoscaler, on purpose.
- Services for the API and the web tier.
- An HTTPRoute (`gateway.httpRoute`) or an Ingress (`ingress`). Both send
  `/api` and `/ics` to the API and `/` to the web tier.
- A ConfigMap with the non-secret `CATALYSTCAL_*` values (`config.*`).
- Options: a LinkKeys RP (`linkkeysRp.enabled`), the act-as keys mount
  (`actAs.enabled`), and a backup CronJob (`backup.enabled`).

The chart refuses `config.devAuthEnabled: true` unless `config.env` is `dev`
or `nonprod`.

### Check the install

```sh
kubectl -n catalystcal get pods
kubectl -n catalystcal port-forward deploy/catalystcal-api 7081:7081
curl http://localhost:7081/readyz
curl https://catalystcal.example.com/api/health
```

## LinkKeys sign-in

catalystcal is a LinkKeys relying party (RP). People sign in at their LinkKeys
identity provider (IDP). The API needs an RP to verify each sign-in.

Set these values (Helm `config.linkkeys.*`):

- IDP domain and IDP URL: the LinkKeys instance where people have accounts.
- RP domain: the domain of the RP that signs the sign-in requests.
  LinkKeys binds each assertion to this audience. When the RP is an
  existing LinkKeys instance (the usual setup), this is the domain of that
  instance. Leave it empty to use the IDP domain.
- The callback URL: `<public URL>/auth/callback`. It must use `https`.
  LinkKeys refuses an `http` callback (`callback_not_https`).

### Claims

catalystcal sends its own claim request with each sign-in. It does not use the
`RP_CLAIMS_CONFIG` of the RP. The request is in
`api/internal/linkkeys/tcp.go` (`CatalystCalClaims`):

| Claim | Kind | Why |
| --- | --- | --- |
| `handle` | required | Shares and delegations find people by handle. |
| `display_name` | optional | The name that catalystcal shows. |
| `email` | optional | Kept for invitations. |

`handle` is required because LinkKeys asks for consent again only when a
required claim is missing from an earlier grant. LinkKeys approves a new
optional claim silently and does not release it. When a person has no
handle, the consent screen asks for one. A person who declines the handle
cannot sign in.

Then select how the API reaches an RP.

### Use an existing LinkKeys instance

This is the usual setup.

Make an API key on that instance. The key needs only the `api_access`
relation:

```sh
linkkeys user create catalystcal-api "catalystcal API" --api-key --relation api_access
```

Put the key in the API Secret as `CATALYSTCAL_LINKKEYS_PKI_API_KEY`.

The API talks to the RP with CSIL-RPC over TCP and TLS. Leave
`linkkeysRp.tcpAddr` and `linkkeysRp.tcpFingerprints` empty to discover
them from DNS (`_linkkeys_apis.<domain>` and `_linkkeys.<domain>` TXT
records), or set them. In the same cluster, use the Service address of the
instance. `deploy/values-squizzlezig.yaml` is an example.

### Run an RP in the cluster

Set `linkkeysRp.enabled: true`. The chart runs a LinkKeys container in RP
mode with its own Service. The API uses
`catalystcal-linkkeys-rp:4987` (for the release name `catalystcal`). After the
first deploy:

1. Open a shell in the RP pod.
2. Run `linkkeys domain init` to make the domain keys.
3. Run `linkkeys user create catalystcal-api "catalystcal API" --api-key`. Save the key
   in `linkkeysRp.apiKey` (or in your existing Secret).
4. Run `linkkeys domain dns-check`. Publish the TXT record that it shows.
5. Put the fingerprints from that record in `linkkeysRp.tcpFingerprints`.
6. Run `helm upgrade` again.

The in-cluster RP has the same shape as the one in the longhouse chart.

## Act-as application keys

A LinkKeys act-as grant lets another application act as a catalystcal user. For
this, catalystcal needs its own LinkKeys application identity and Ed25519 signing
keys. The API reads them from a JSON file (`CATALYSTCAL_ACT_AS_APP_KEYS_FILE`).
The file contains private key seeds. Treat it as a secret.

Without the file, the API works, but act-as grants do not.

1. Register catalystcal as an application with your LinkKeys domain. Save the
   identity and the key seeds in the JSON layout that the catalystcal API reads.
2. Make a Secret from the file:

   ```sh
   kubectl -n catalystcal create secret generic catalystcal-act-as \
     --from-file=app-keys.json=./act-as-app-keys.json
   ```

3. Set the values:

   ```yaml
   actAs:
     enabled: true
     existingSecret: catalystcal-act-as
     secretKey: app-keys.json
   ```

The chart mounts the file read-only at
`/run/secrets/catalystcal/act-as/app-keys.json` and sets
`CATALYSTCAL_ACT_AS_APP_KEYS_FILE`. `actAs.appKeysJson` can hold the JSON inline,
but then the key seeds are in your values file. Do not use it in
production.

With Docker Compose, mount the file read-only and set
`CATALYSTCAL_ACT_AS_APP_KEYS_FILE` (see the comments in `docker-compose.yml`).

## Backups

All catalystcal data is in one SQLite database. In WAL mode, recent changes are in
`catalystcal.db-wal` until SQLite moves them into `catalystcal.db`. A plain file copy of
`catalystcal.db` while the API runs can be incomplete or damaged. Use the SQLite online backup API
(`sqlite3 .backup`). It makes a consistent copy while the API writes.

### Helm backup CronJob

Set `backup.enabled: true`. On each run (`backup.schedule`, default 03:17
UTC each day), the job:

1. Runs `sqlite3 /data/catalystcal.db ".backup ..."` against the live database.
2. Runs `PRAGMA integrity_check` on the copy. The job fails if the check
   fails.
3. Writes `catalystcal-<UTC time>.db` to its own PVC (`backup.persistence`).
4. Deletes the oldest copies, and keeps `backup.keep` (default 14).

The job must run on the node of the API pod. SQLite WAL readers share
memory with the writer through the `-shm` file, and the data volume is
ReadWriteOnce. The chart adds a required pod affinity for this. If the API
pod is not running, the job cannot start.

The backup PVC is in the same cluster as the database. Copy the backups to
another place too, for example with a job that uploads the newest file to
object storage. To copy one file by hand, start a temporary pod that mounts
the backup PVC (`catalystcal-backup` for the release name `catalystcal`), then use `kubectl cp` from that pod.
The API image has no shell, so `kubectl cp` from the API pod does not work.

### Manual backup

With the `sqlite3` CLI on a host that can open the file (the same host as
the API):

```sh
sqlite3 /path/to/catalystcal.db ".backup '/path/to/catalystcal-backup.db'"
sqlite3 /path/to/catalystcal-backup.db "PRAGMA integrity_check;"
```

With Docker Compose:

```sh
docker run --rm -v catalystcal_catalystcal_data:/data -v "$PWD":/backup keinos/sqlite3:3.53.4 \
  sqlite3 /data/catalystcal.db ".backup '/backup/catalystcal-backup.db'"
```

The volume name has the Compose project name as a prefix. Run
`docker volume ls` to find it.

### Restore

1. Stop the API. Helm: `kubectl -n catalystcal scale deploy/catalystcal-api --replicas=0`.
   Compose: `docker compose stop api`.
2. Mount the data volume in a temporary pod or container.
3. Remove `catalystcal.db-wal` and `catalystcal.db-shm`.
4. Copy the backup file to `/data/catalystcal.db`. Make sure user 65532 owns it.
5. Start the API. Helm: `helm upgrade` (it sets 1 replica again), or
   `kubectl -n catalystcal scale deploy/catalystcal-api --replicas=1`. Compose:
   `docker compose start api`.

Test a restore before you need one.

## Upgrades

1. Read the release notes.
2. Make a backup (see above).
3. Upgrade:

   ```sh
   helm upgrade catalystcal ./helm_chart -n catalystcal -f my-values.yaml \
     --set image.api.tag=<version> --set image.web.tag=<version>
   ```

   Without the `--set` values, the images use the chart `appVersion`.

4. Watch the API pod: `kubectl -n catalystcal rollout status deploy/catalystcal-api`.

The API Deployment uses `Recreate`. The old pod stops before the new pod
starts, so two API processes never write the same file. The API is down
for a short time during each upgrade. The web tier rolls normally.

Database migrations are embedded in the API binary. Set
`api.migrateInitContainer: true` to run `catalystcal migrate` in an init container
before `catalystcal serve` starts. If a migration fails, the new pod does not start.
Restore the backup and deploy the old version to go back. Do not run an old
API against a database that a newer version changed.

With Docker Compose:

```sh
git pull
docker compose up -d --build
```

## Release images

CI builds and pushes the images on each release tag:

- `containers.catalystsquad.com/public/catalystcal/api:<version>` and `:latest`
- `containers.catalystsquad.com/public/catalystcal/web:<version>` and `:latest`

The packaged Helm chart is attached to the GitHub release. Use a version
tag in production, not `latest`.
