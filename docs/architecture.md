# Architecture

## Services

| Service | Image | Port | What it does |
|---|---|---|---|
| `web` | nginx-unprivileged 1.30.6 (alpine-slim), uid 101 | 3000 → 8080 | serves the React build, proxies `/api` to `api` |
| `api` | Go 1.26.9 → distroless static nonroot, uid 65532 | 8080 | HTTP API |
| `db` | PostgreSQL 18.6 (alpine) without `gosu`, uid 70 | not published | data |

The services start in a healthcheck chain: `db` → `api` → `web`.

The frontend is the separate repository `jana-address-web`, a submodule in `web/` (see its
README: [jana-address-web](https://github.com/jana-address/jana-address-web)).

## Data flow

1. The operator works in the UI on port 3000; nginx in `web` proxies `/api` to `api`.
   Clients can also call `api` directly on port 8080.
2. `api` parses each line with the rules and the reference data embedded in the binary: the
   KATO snapshot, the OpenStreetMap street directory and the precinct tables (see
   [reference-data.md](reference-data.md)). Parsing does not wait for the database or the
   network.
3. `db` stores the KATO versions and the sync log (see
   [operations.md](operations.md#kato-in-the-database-and-migrations)). If the database is
   down, parsing still works and `/api/v1/health` reports `"db":"down"`.

## Security

- The database sits in an internal network with no way out.
- All containers run as non-root, with `cap_drop: ALL` and `no-new-privileges`; `api` and
  `web` also have a read-only file system.
- No external requests from the UI or the API: fonts are local, and by default
  (`OFFLINE=true`) the API does not go to the network.
- Base images are pinned by multi-arch digest; GitHub Actions are pinned by commit SHA,
  not by tag.

## Versions

We take stable versions without known vulnerabilities, but not the newest ones. If a
vulnerability fix exists only in a newer version, we take that version. Base images are
pinned by multi-arch digest.

| Component | Version | Newest on 17.10.2026 |
|---|---|---|
| Go | 1.26.9 | 1.27.x |
| pgx | v5.10.0 | v5.11.0 |
| golang.org/x/text | v0.41.0 (the minimum with the fixes for GO-2026-5970 and GO-2026-6629) | v0.43.0 |
| PostgreSQL | 18.6 | 19 (RC) |
| React | 19.2.9 | 19.3.x |
| Vite | 8.2.4 | 8.3.x |
| TypeScript | 6.0.3 | 7.0 |
| Node (build only) | 24.22.0 LTS | 26.x |
| nginx | 1.30.6 (stable) | 1.31 (mainline) |

Vulnerability checks in CI: `govulncheck` for Go and `npm audit --audit-level=high` for the
web part.
