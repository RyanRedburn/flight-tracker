---
name: development-practices
description: Apply flight-tracker Go service conventions for package layering, style, testing, config, and verification. Use when implementing features, fixing bugs, refactoring, reviewing code, adding files, or otherwise changing this repository.
---

# Development practices

Read this skill before changing code. Then read the specialist skill that matches the work:

- HTTP handlers, routes, query parsing, or Swagger → [http-api](../http-api/SKILL.md)
- Ingest adapters, jobs, or workers → [ingest-jobs](../ingest-jobs/SKILL.md)
- Store interface, SQL, or migrations → [store-and-migrations](../store-and-migrations/SKILL.md)

Details: [architecture.md](architecture.md), [style.md](style.md), [testing.md](testing.md).

## What this service is

In-process Go HTTP API plus poll-based background workers. Handlers queue jobs; workers download, parse, and load Postgres. Stats/outlook endpoints read aggregated flight data.

## Package map

| Path | Owns |
| --- | --- |
| `cmd/server` | Process wiring only (config, store, ingest services, processor, worker, HTTP server) |
| `internal/api` | Chi router, handlers, query parsing, middleware |
| `internal/operator` | Worker loop, job processor, per-type `JobHandler` |
| `internal/ingest` | Month-range expansion; shared CSV/HTTP helpers; provider adapters (`bts`, `iem`, `ourairports`) |
| `internal/store` | `Store` interface, SQL constants, query filters |
| `internal/store/postgres` | pgx implementation, COPY replace |
| `internal/store/storetest` | Scenario stub used by unit tests |
| `internal/model` | Domain types and JSON request validation |
| `internal/config` | Environment variables |
| `internal/database` | Driver factory (`postgres` only) |
| `migrations/postgres` | golang-migrate SQL |
| `docs/external`, `docs/full` | Generated OpenAPI — never edit by hand |

## Dependency rules

- Handlers and operator code depend on `store.Store`, never on `internal/store/postgres`.
- SQL lives in `internal/store/queries.go`. Query construction may live in `internal/store/postgres`. Handlers do not embed SQL.
- HTTP stays in `internal/api`. Ingest download/parse stays in `internal/ingest/<provider>`. Job claim/complete stays in `internal/operator`.
- `cmd/server` is the composition root. New long-lived collaborators are constructed there and passed in.
- Prefer extending an existing package over adding a new top-level `internal/` package.

## Change checklist

1. Match neighboring files for imports, error wrapping, JSON names, and test style.
2. Do not add one-line wrapper functions with no real utility. Shims are allowed (see [style.md](style.md)).
3. If `store.Store` gains a method, update `internal/store/storetest/stub.go` in the same change (`var _ store.Store = (*Stub)(nil)` must compile).
4. If env vars change, update `internal/config/config.go`, the README env table, and `.env.example` when Compose/local defaults are involved.
5. If user-visible API or ingest behavior changes, update README examples/behavior notes.
6. Do not bump pinned tool versions (`swag` `v1.16.6` in `cmd/server/main.go` / Makefile / CI; golangci-lint `v2.12.2`) unless the task is to bump them.

## Verify before finishing

Run from the repo root:

```bash
make lint
make test
```

Also run `make swagger` when handler swag comments, router paths, or exported API models change. Commit the regenerated files under `docs/`. CI fails on swagger drift.

Do not add `//nolint` to silence a new issue; fix the code unless an existing neighboring exception is clearly the project pattern.
