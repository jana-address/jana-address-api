# Development

## Running locally

```bash
# API locally (needs Go 1.26.9; an older Go downloads it by the toolchain directive)
go test ./...
DATABASE_URL=postgres://jana:jana_local_only@localhost:5432/jana?sslmode=disable go run ./cmd/server

# tests in a container
docker build --target test .

# frontend: see web/README.md
```

The frontend: `web/README.md`, or [jana-address-web](https://github.com/jana-address/jana-address-web) on GitHub.

## Tests

- Unit tests and the evaluator: `go test ./...`; the metric run is described in
  [quality.md](quality.md#running-the-evaluation).
- The database integration test runs only if `TEST_DATABASE_URL` is set:

  ```bash
  TEST_DATABASE_URL=postgres://jana:jana_local_only@localhost:5432/jana?sslmode=disable go test ./...
  ```

## CI

GitHub Actions (`.github/workflows/ci.yml`) runs on every push to `main` and on pull requests:

- **test:**
  - `go mod verify`, `gofmt`, `go vet`;
  - `go test -race` with coverage against PostgreSQL 18.6 (service container);
  - `govulncheck`.
- **build:**
  - `docker compose config`;
  - build of the `api` and `db` images;
  - smoke test: `docker compose up --wait db api`, then `/api/v1/health` must return `"db":"ok"`.

Actions are pinned by commit SHA, not by tag.

## Tools

| Command | What it does | Details |
|---|---|---|
| `go run ./cmd/osmref fetch` | downloads streets, houses with an address, ЖК and microdistricts for each district or city polygon from the Overpass API into `data/raw/osm/*.json` | [reference-data.md](reference-data.md#how-it-is-built) |
| `go run ./cmd/osmref build` | builds `internal/parser/dict/osm/` from `data/raw/osm/*.json` | [reference-data.md](reference-data.md#how-it-is-built) |
| `go run ./cmd/kato fetch` | updates the KATO snapshot in `internal/kato/snapshot/` | [reference-data.md](reference-data.md#kato) |
| `go run ./cmd/synth …` | generates a synthetic set | [quality.md](quality.md#running-the-evaluation) |
| `docker compose run --rm --no-deps -T api synth` | the same 10 000-line set from the image, without Go | [quality.md](quality.md#running-the-evaluation) |
| `docker compose run --rm api migrate` | applies the database migrations | [operations.md](operations.md#kato-in-the-database-and-migrations) |

## Updating the web submodule

After changes in the frontend, update the submodule pointer:

```bash
git -C web pull origin main && git add web && git commit -m "feat(web): bump submodule"
```
