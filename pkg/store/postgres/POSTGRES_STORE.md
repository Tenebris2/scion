# PostgreSQL Store — Implementation Guide

This document captures the non-obvious decisions, gotchas, and workflow for the
PostgreSQL `store.Store` implementation so any engineer (or Claude session) can
pick up work quickly.

---

## Package layout

```
pkg/store/postgres/
  postgres.go                 # PostgresStore struct, New(), Ping(), Close()
  schema.go                   # Full DDL — CREATE TABLE IF NOT EXISTS for every table
  driver.go                   # Registers the pgx stdlib driver ("pgx")
  helpers.go                  # boolToInt, marshalJSON, unmarshalJSON, nullableString
  <domain>.go                 # One file per store domain (agents, projects, …)
  postgres_conformance_test.go # Integration test — skips if SCION_TEST_POSTGRES_DSN unset

pkg/store/storetest/
  conformance.go              # RunAll(t, newStore) — 21 backend-agnostic subtests

pkg/store/sqlite/
  conformance_test.go         # Wires RunAll against an in-memory SQLite store
```

---

## Critical: pgx v5 SMALLINT encoding

The schema uses `SMALLINT` (not `BOOLEAN`) for all boolean columns so that the
same DDL works on both SQLite (no BOOLEAN) and Postgres.

pgx v5 uses the binary wire protocol by default. It can only encode a Go `int16`
into a Postgres `int2`/SMALLINT — **not** `bool`, `int`, or `int64`.

**Rule: every boolean written to a SMALLINT column must be wrapped with `boolToInt()`.**

```go
// helpers.go
func boolToInt(b bool) int16 {
    if b { return 1 }
    return 0
}
```

On the read side, scan into `int` (or `sql.NullInt64`) and convert with `!= 0`:

```go
var sensitive int
rows.Scan(&sensitive)
envVar.Sensitive = sensitive != 0
```

Bare `bool` values in INSERT/UPDATE args will panic at runtime with:
> `unable to encode false into binary format for int2`

This affected these columns initially (all now fixed): `sensitive`, `secret`,
`detached`, `web_pty_enabled`, `auto_provide`, `locked` (templates +
harness_configs).

---

## Running tests

### Conformance suite (all backends)

```bash
# SQLite — runs in-memory, no setup
go test ./pkg/store/sqlite/...

# Postgres — requires a live DSN
SCION_TEST_POSTGRES_DSN="postgres://scion:scion@localhost:5432/scion_test?sslmode=disable" \
  go test ./pkg/store/postgres/... -run TestConformance -v

# Self-hosted (no local postgres required)
make test-postgres
```

`make test-postgres` creates a Docker network, runs `postgres:16-alpine`, waits
for readiness, then runs the Go test container against it. Cleans up on exit.

### All tests

```bash
make test          # full suite (requires CGO for SQLite)
make test-fast     # -tags no_sqlite (faster, no CGO)
```

---

## Local dev server with Postgres

```bash
make dev-postgres
```

This runs `docker compose -f docker-compose.dev-postgres.yml down && up`, which:

1. Starts `postgres:16-alpine` (healthcheck-gated)
2. Runs the scion server via `go run ./cmd/scion server start --foreground --host 0.0.0.0`
   inside a `golang:1.25-alpine` container with:
   - The repo mounted at `/workspace`
   - The Docker socket mounted (server requires a container runtime)
   - `docker-cli` installed via apk
   - `SCION_SERVER_DATABASE_DRIVER=postgres`
   - `SCION_SERVER_DATABASE_URL=postgres://scion:scion@postgres:5432/scion_dev?sslmode=disable`

Web UI: http://localhost:8080  
Hub API: http://localhost:9810  
Health: `curl http://localhost:8080/healthz`

Logs: `docker compose -f docker-compose.dev-postgres.yml logs -f server`

The "Slow request" WARN for `/events` is expected — it's a long-poll SSE connection.

---

## Adding a new domain

1. Create `pkg/store/postgres/<domain>.go` — implement all methods from the
   relevant sub-interface in `pkg/store/store.go`.
2. Add test coverage in `pkg/store/storetest/conformance.go` under `RunAll`.
3. Remember `boolToInt()` for every boolean INSERT/UPDATE arg.
4. No schema changes needed unless the domain has a new table — add DDL to
   `schema.go` if so.

---

## Wiring (how postgres is selected at runtime)

`cmd/server_foreground.go → initStore()`:

```go
case "postgres":
    pgStore, err := postgres.New(cfg.Database.URL)
    pgStore.Migrate(ctx)   // applies schema.go DDL
    pgStore.Ping(ctx)
    return pgStore, nil
```

Env vars:
- `SCION_SERVER_DATABASE_DRIVER=postgres`
- `SCION_SERVER_DATABASE_URL=<dsn>`

---

## Known limitations / stubs

`GetEffectiveGroupsForAgent` and `CheckDelegatedAccess` return `nil, nil` /
`false, nil` in the postgres store — same as the SQLite base store. Full
group/policy logic is handled by `entadapter.CompositeStore`, which wraps an
Ent-backed layer on top of the base store. The Ent adapter currently only has
a Postgres path (`OpenPostgres`) but is not yet wired for the postgres case in
`initStore`. This is a future task.
