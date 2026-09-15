---
name: store-and-migrations
description: Change the flight-tracker Store interface, Postgres SQL, COPY loads, query filters, and golang-migrate schema. Use when working on internal/store, internal/store/postgres, migrations/postgres, or adding database methods.
---

# Store and migrations

Also follow [development-practices](../development-practices/SKILL.md).

## Split

| Location | Put |
| --- | --- |
| `internal/store/store.go` | `Store` interface |
| `internal/store/queries.go` | Raw SQL string constants (`Query*`) |
| `internal/store/*_agg.go`, `flight_date.go`, … | Filter structs and shared query helpers used by the interface |
| `internal/store/postgres` | Execution, transactions, COPY, dynamic `WHERE` builders |
| `internal/store/storetest/stub.go` | Test stub — every interface method |
| `migrations/postgres` | Schema changes |

Handlers and ingest services call the interface. Only `internal/database` and `postgres.Open` construct the concrete type.

## Adding a store method

1. Add the method to `store.Store`.
2. Add `Query*` SQL (parameterized `$1`, `$2`, … — no string-concatenated user input).
3. Implement on `postgres.Store`.
4. Add `FooFn` + method on `storetest.Stub`. Unset Fn must `panic("unexpected call: Foo")`.
5. Tests: stub consumers in handler/operator tests; query-builder tests in `postgres` if SQL is assembled dynamically.

`var _ store.Store = (*Stub)(nil)` must keep compiling.

## Loads (ingest)

Bulk replace goes through `replaceTable` / `replaceTables` in `internal/store/postgres/copy.go`:

- Delete the target slice (month or full table) in a transaction.
- `COPY` rows with the adapter's canonical column list.
- Flight/weather observations are **per month**. Reference tables are **full replace**. Weather stations replace catalog + mapping together.

Do not row-by-row `INSERT` for ingest loads.

## Read queries

Keep templates in `queries.go`. Optional filters are applied in `postgres` builders (see `buildRouteStatsQuery` / `buildCarrierStatsQuery`): replace a placeholder, append args, never interpolate values into SQL.

Stats/outlook aggregation belongs in SQL (or small helpers in `store`/`postgres`), not in HTTP handlers.

Sentinels: `store.ErrNotFound`, `store.ErrJobStatusConflict`. Map `sql.ErrNoRows` to `ErrNotFound` at the postgres boundary.

## Migrations

golang-migrate, sequential numbering:

```text
migrations/postgres/NNNNNN_short_name.up.sql
migrations/postgres/NNNNNN_short_name.down.sql
```

`NNNNNN` is the next unused six-digit prefix in `migrations/postgres`. Each change needs a working `down`. Prefer additive indexes/columns; ingest tables are not treated as user-authored relational data.

Migrations run automatically in `postgres.Open` on server start (`MIGRATIONS_PATH`). For local Compose:

```bash
make migrate-up
make migrate-version
```

Do not hand-edit already-applied files on `main`; add a new pair.

There are **no** live-DB tests. Exercise SQL builders and COPY helpers with unit tests. Manual check: `docker compose up -d postgres` then `go run ./cmd/server`.

## Driver

Postgres via pgx stdlib (`DATABASE_DRIVER=postgres` only). Adding another driver is out of scope unless explicitly requested.
