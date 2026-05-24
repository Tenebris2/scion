# PostgreSQL Store — Outstanding Tasks

Track work remaining to make the Postgres backend production-complete.
See `POSTGRES_STORE.md` for architecture context.

---

## #8 — Wire Ent adapter (CompositeStore) for PostgreSQL

**Status:** done  
**File:** `cmd/server_foreground.go:638-663`

Wired `entc.OpenPostgres` + `entc.AutoMigrate` + `entadapter.NewCompositeStore` into the postgres case of `initStore()`. Both `pgStore` and `entClient` are closed on any failure path.  
`MigrateGroveToProjectData` is intentionally omitted — it is SQLite-only and Postgres starts clean.

---

## #9 — Add Ent adapter conformance tests for Postgres

**Status:** done  
**Files:** 
- Suite runners: `pkg/store/entadapter/{composite,group_store,policy_store}_suite_test.go`
- Postgres tests: `pkg/store/entadapter/*_postgres_test.go`
- Test utilities: `pkg/store/entadapter/postgres_testutils_test.go`

Refactored all existing entadapter tests into suite-runner pattern to share test bodies across SQLite and Postgres backends. Created `_postgres_test.go` variants for group, policy, and composite stores that skip unless `SCION_TEST_POSTGRES_DSN` is set. Each subtest truncates and reseeds Ent tables via `truncateEntTables()` helper.

---

## #10 — Investigate MigrateGroveToProjectData for Postgres

**Status:** done  
**Conclusion:** SQLite-only. `//go:build !no_sqlite`, hardcoded `"sqlite"` driver, uses `randomblob()` and SQLite pragmas. A no-op stub exists for non-SQLite builds. Postgres starts clean — no legacy grove data to migrate. Do not add to the Postgres path.

---

## #11 — Add Postgres to CI pipeline

**Status:** done  
**File:** `.github/workflows/ci.yml`

Added `make test-postgres` step after the existing `make test-fast` step. Single `ubuntu-latest` job, no matrix strategy complications. Docker is available by default on GitHub Actions runners.

---

## #12 — Debug logging for the PostgreSQL store

**Status:** done  
**Files:** `pkg/store/postgres/postgres.go`, `pkg/store/postgres/debug.go`

Added opt-in SQL query/result debug logging.  
Enable with env var: `SCION_POSTGRES_DEBUG=1`  
Or programmatically: `store.SetDebug(true)`  
See `debug.go` for implementation.

---

## #13 — Separate Ent tables into dedicated Postgres schema

**Status:** done  
**Files:** 
- `pkg/ent/entc/client.go` — added `OpenPostgresInSchema`, `appendSchemaToPostgresDSN`
- `cmd/server_foreground.go` — use `OpenPostgresInSchema(ctx, dsn, "ent")`
- `pkg/store/entadapter/postgres_testutils_test.go` — use schema-qualified table names
- `pkg/store/entadapter/*_postgres_test.go` — use `OpenPostgresInSchema`

**Reason:** The main postgres store creates tables with `id TEXT`, while Ent uses `id UUID`. AutoMigrate would try to `ALTER TABLE ... CHANGE COLUMN TYPE`, which fails without a `USING id::uuid` cast.

**Solution:** Create a dedicated `ent` schema in Postgres. All Ent tables (including shadow records for users, agents, projects) live in `ent.*`, while the main store tables stay in `public.*`. This avoids schema conflicts and eliminates the type mismatch.

Ent client connects with `search_path=ent`, so all queries operate within the `ent` namespace. Tests use schema-qualified names (`ent.table_name`) in truncation queries.
