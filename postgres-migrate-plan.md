# Plan: PostgreSQL `Store` Driver for SCION Hub

## Context

The SCION Hub persists all state through the `store.Store` interface
(`pkg/store/store.go`) — ~201 methods across 24 embedded sub-interfaces. The
only implementation today is SQLite (`pkg/store/sqlite/`, `sqlite.go` ~6017
lines + 10 per-domain files). SQLite's single-writer WAL model is the
scaling ceiling: it cannot serve multiple Hub instances against one shared
database. This plan adds a PostgreSQL implementation so the Hub can scale
horizontally (multi-instance, MVCC concurrency, managed hosting).

Scope decisions (confirmed with user):
- **Greenfield** — new deployments start on Postgres; no SQLite→Postgres data
  migration tool. Schema is a single squashed baseline.
- **Shared conformance test suite** — the existing ~120 SQLite tests are
  refactored into a reusable suite both drivers run, guaranteeing parity.
- **Driver: `jackc/pgx/v5`** via the `database/sql` stdlib adapter, so all
  existing `rows.Scan` / `sql.NullString` code ports unchanged.

The `Store` interface, `DatabaseConfig{Driver, URL}` (`pkg/config/hub_config.go`,
comment already says "sqlite, postgres"), and the Ent layer
(`entc.OpenPostgres` already exists) need **no changes** — they are already
dialect-agnostic.

## Approach

New package `pkg/store/postgres/` mirroring `pkg/store/sqlite/` file-for-file.
The 53 incremental SQLite migrations are **squashed** into one Postgres-dialect
baseline schema (the post-V53 state). Queries are **hand-ported** (no regex
`?`→`$N` rewriter — string literals and JSON paths contain `?` and would be
silently corrupted).

## Phases

### P0 — Scaffolding (~0.5d)
- Add `github.com/jackc/pgx/v5` to `go.mod`.
- Create `pkg/store/postgres/`:
  - `postgres.go` — `PostgresStore` struct wrapping `*sql.DB`, `New(dsn)`,
    `Close`, `Ping`, `Migrate`. No build tag (mirrors `sqlite.go`, which has
    none). Pool limits 16/8 (Postgres handles concurrency, unlike SQLite's 4/4).
  - `driver.go` — `//go:build !no_postgres` + blank import
    `_ "github.com/jackc/pgx/v5/stdlib"` (registers `"pgx"`).
  - `helpers.go` — `ph(start, n)` IN-clause placeholder expander; copy
    `marshalJSON`/`unmarshalJSON` helpers from sqlite.
- Stub all ~201 interface methods returning a sentinel `errNotImplemented` so
  the package compiles and satisfies `store.Store`.

### P1 — Baseline schema + migration runner (~1.5–2d)
- Derive one squashed `migrationV1` const = the post-V53 schema in Postgres
  dialect. Translation rules:
  - `TEXT PRIMARY KEY` (app-generated UUID strings) → keep `TEXT`.
  - Boolean-ish `INTEGER DEFAULT 0/1` (`detached`, `web_pty_enabled`,
    `stalled_from_activity`, …) → keep as `SMALLINT` so Go `int` scan sites are
    unchanged (BOOLEAN conversion deferred as tech debt).
  - `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` → `TIMESTAMPTZ DEFAULT NOW()`.
  - `agents.ancestry` → `JSONB` (required for `jsonb_array_elements_text` in
    `ListAgents`); `labels`/`annotations` → `JSONB`; opaque blobs stay `TEXT`.
  - `email ... UNIQUE COLLATE NOCASE` (`sqlite.go:1186,1351`) → `email TEXT` +
    `CREATE UNIQUE INDEX ON (lower(email))` (avoids `citext` extension dep).
  - FKs / `ON DELETE CASCADE` port unchanged.
- Migration runner: mirror sqlite's loop but **string-only** (the `func`-typed
  `migrateV50` grove→project rename is irrelevant to a fresh schema and is
  dropped). Keep `schema_migrations(version INTEGER PK, applied_at TIMESTAMPTZ)`;
  baseline = 1, future migrations from 2.
- Wrap the whole `Migrate()` body in `pg_advisory_lock(<const>)` so racing Hub
  replicas cannot apply migrations concurrently.
- **Verify on a real Postgres** that store tables do not collide with Ent's
  tables (`groups`, `policies`, `users`, Ent's `projects` mirror).

### P2 — Core domain port (~2–3d)
Port Agent / Project / RuntimeBroker methods in `postgres.go`. Per-pattern
translation (locations confirmed in `sqlite.go`):
- `?` → `$1,$2,…` positional placeholders (pervasive).
- `INSERT OR IGNORE` (`:1036,:4006`) → `INSERT … ON CONFLICT DO NOTHING`.
- `INSERT OR REPLACE` (`:4313`) → `INSERT … ON CONFLICT (…) DO UPDATE SET …`.
- `json_each(ancestry)` (`:1755`) → `jsonb_array_elements_text(ancestry)`.
- `json_array(x)` (`:978`) → `jsonb_build_array(x)`.
- `datetime('now')` → `NOW()`; `SUBSTR/INSTR` → `SUBSTRING/POSITION`.
- `COALESCE(col,'')` in a UNIQUE index — **no change needed**, Postgres
  supports expression indexes.
- Hardest query — `ListAgents` (`sqlite.go:1710`): dynamic `conditions`/`args`
  builder. Each appended `?` becomes `$N` via a running counter; `IN (...)`
  expansions use `ph()`; `AncestorID` branch →
  `EXISTS (SELECT 1 FROM jsonb_array_elements_text(ancestry) v WHERE v = $N)`.
- Audit all other dynamic builders: `grep -n "append(args" pkg/store/sqlite/`
  and apply the same counter pattern.

### P3 — Remaining domains (~2–3d)
Port the per-domain files: `messages.go`, `schedule.go`, `scheduled_event.go`,
`brokersecret.go`, `gcp_service_account.go`, `github_installation.go`,
`notification.go`, `maintenance.go`, `project_sync_state.go` — same
translation rules as P2.

### P4 — Shared conformance suite (~2d, parallel with P3)
- Create `pkg/store/storetest/conformance.go`, `package storetest`, exporting
  `Run(t *testing.T, newStore func(*testing.T) store.Store)`.
- Move every test body that uses **only** the exported `store.Store` interface
  (the majority of the 120) into `storetest`, parameterized over the factory.
- Tests that reach into `*SQLiteStore.db` or use `PRAGMA` (~24 spots in
  `sqlite_test.go`, `notification_test.go`, `scheduled_event_test.go`) stay in
  `package sqlite` as a residual dialect-specific file.
- Add `pkg/store/sqlite/sqlite_conformance_test.go` (`//go:build !no_sqlite`)
  calling `storetest.Run` with a `New(":memory:")` factory.
- Add a `time.Time` UTC round-trip assertion to the suite (catches
  `TIMESTAMP` timezone drift).

### P5 — Postgres tests + CI (~1–1.5d)
- `pkg/store/postgres/postgres_conformance_test.go` (`//go:build !no_postgres`)
  — `storetest.Run` with a Postgres factory that `t.Skip`s when
  `SCION_TEST_POSTGRES_DSN` is unset (so local `go test ./...` still passes).
- Makefile target `test-postgres` running the postgres + storetest packages
  with `SCION_TEST_POSTGRES_DSN` set. Leave `make test-fast` untouched.
- CI: add a job with a `postgres:16` service container running `test-postgres`.
- Validate the build-tag matrix compiles: default, `no_sqlite`, `no_postgres`,
  `no_sqlite,no_postgres`.

### P6 — Wiring + e2e (~0.5–1d)
- Add a `case "postgres":` to `initStore()` in `cmd/server_foreground.go:634`,
  mirroring the sqlite case: `postgres.New` → `Migrate` → `entc.OpenPostgres`
  (same `cfg.Database.URL`, single DB) → `entc.AutoMigrate` →
  `entadapter.NewCompositeStore(pgStore, entClient)` → `Ping`. Skip
  `MigrateGroveToProjectData` (greenfield).
- Add the `pkg/store/postgres` import.
- Manual e2e: run `scion server` with `Database.Driver=postgres` against a
  local Postgres; exercise agent create/list/update.

## Critical Files
- `pkg/store/sqlite/sqlite.go` — source of truth for schema + queries to port.
- `pkg/store/sqlite/{messages,schedule,scheduled_event,brokersecret,gcp_service_account,github_installation,notification,maintenance,project_sync_state}.go` — per-domain sources.
- `pkg/store/store.go` — the interface contract (unchanged).
- `cmd/server_foreground.go:634` — `initStore()` driver switch.
- `pkg/ent/entc/client.go` — `OpenPostgres` already exists (unchanged).
- `pkg/store/entadapter/composite.go` — dialect-agnostic wrapper (unchanged).
- New: `pkg/store/postgres/*`, `pkg/store/storetest/conformance.go`.

## Verification
- `go build ./...` and the 4-way build-tag matrix all compile.
- `make test` (with SQLite) — sqlite conformance suite + residual sqlite tests pass.
- `make test-postgres` against a real `postgres:16` — full conformance suite
  passes on the Postgres driver, proving behavior parity with SQLite.
- Manual: `scion server` with `Database.Driver=postgres` boots, runs
  migrations, and serves agent CRUD; `scion server status` reports healthy.

## Effort & Risks
Total ~10–14 days. P1→P2→P3 sequential; P4 parallel with P3; P5 needs P3+P4;
P6 last.

Risks:
- Hidden dynamic-filter queries beyond `ListAgents` — mitigate by grepping
  `append(args` early in P2.
- `TIMESTAMP`/UTC discipline — mitigate with the round-trip assertion in P4.
- Ent vs store table-name collision — verify on real Postgres in P1.
- `SMALLINT`-as-boolean — accepted tech debt; BOOLEAN conversion is a later
  follow-up requiring a scan-site audit.



