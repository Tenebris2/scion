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

**Status:** pending  
**Files:** `pkg/store/entadapter/composite_test.go`, `group_store_test.go`, `policy_store_test.go`

All existing entadapter tests run against SQLite only (`//go:build !no_sqlite`).
Add a parallel `_postgres_test.go` file that runs the same suites when
`SCION_TEST_POSTGRES_DSN` is set.

**Pattern to follow:** `pkg/store/postgres/postgres_conformance_test.go`

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
