# Operations

## Offline mode

The network is needed only for the first image build:

```bash
docker compose build
```

After that, `docker compose up` works offline. Fonts are local; there are no external
requests from the UI or the API.

## KATO in the database and migrations

The parser works from the KATO snapshot embedded in the binary (see
[reference-data.md](reference-data.md#kato)). The database keeps the KATO versions.

- **Migrations from scratch.** At start the API waits for PostgreSQL, applies the SQL
  migrations from `internal/db/migrations/` (table `schema_migrations`, advisory lock) and,
  if the database is empty, writes the snapshot as the first KATO version. By hand:
  `docker compose run --rm api migrate`.
- **Versions in the database.** `kato_version` (one active), `kato_object` (classifier
  rows), `kato_sync_log` (every check).
- **Refresh.** By default `OFFLINE=true`: the API does not go to the network, and the final
  jury run has no internet. With `OFFLINE=false` the API, once every `KATO_REFRESH_INTERVAL`
  (default `168h`), opens the classifier page, finds the link to the latest xlsx and compares
  the sha256. If the file is new, the API checks it (≥ 10 thousand rows, 20 regions, unique
  codes, difference from the previous version ≤ 15%), saves it as a new version and swaps the
  parser's directory without a restart. On error the old version stays, and the error is
  visible in `/api/v1/health` and in `kato_sync_log`.

  ```bash
  OFFLINE=false docker compose up
  ```
- **Update the snapshot in the repository:** `go run ./cmd/kato fetch`.

## Environment variables

A `.env` file is optional: every variable has a default (see `.env.example` and
`docker-compose.yml`).

| Variable | Default | Meaning |
|---|---|---|
| `WEB_PORT` | `3000` | host port of the UI |
| `API_PORT` | `8080` | host port of the API |
| `APP_VERSION` | `0.1.0` | API version, passed to the image build as `VERSION` and shown in `/api/v1/health` |
| `POSTGRES_DB` | `jana` | database name |
| `POSTGRES_USER` | `jana` | database user |
| `POSTGRES_PASSWORD` | `jana_local_only` | database password (the database is on an internal network only) |
| `OFFLINE` | `true` | `true`: only the KATO snapshot built into the image; `false`: check stat.gov.kz every `KATO_REFRESH_INTERVAL` |
| `KATO_REFRESH_INTERVAL` | `168h` | how often to check for a new KATO version; at least `1h` |
| `KATO_PAGE_URL` | `https://stat.gov.kz/ru/classifiers/statistical/21/` | the classifier page |
| `HTTP_ADDR` | `:8080` | listen address of the API inside the container |
| `DATABASE_URL` | `postgres://db:5432?sslmode=disable` in compose | database URL; the credentials come from `PGUSER`/`PGPASSWORD`/`PGDATABASE` |

## Health checks

| Check | What it means |
|---|---|
| `GET /livez` | `ok` if the process is alive |
| `GET /readyz` | 200 when the reference data is loaded and the API can parse addresses; the database is not needed for this |
| `GET /api/v1/health` | `status` (`ok` or `degraded` when the database is unavailable), `db`, the KATO version in `directory`, the OSM street directory and its holdout numbers in `streets` |

In `docker-compose.yml`, `db` is checked with `pg_isready`, `api` with `/app/server
healthcheck`, and `web` with a request to its own `/healthz`. The services start in the chain
`db` → `api` → `web`.

## Organizers' data

`train.json`, `sample_input.json` and `sample_output.json` must not leave the hackathon. They
are in `.gitignore` and `.dockerignore`, so they get into neither git nor the images. Keep
them outside the repository.

## Troubleshooting

- **`unable to prepare context: path "…/web" not found` or an empty `web/` folder.** The
  submodule is not downloaded; run `git submodule update --init`.
- **Port in use.** Run `WEB_PORT=3001 API_PORT=8081 docker compose up`.
- **Start with a clean database.** `docker compose down -v` deletes the data volume.
