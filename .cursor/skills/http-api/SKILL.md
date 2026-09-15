---
name: http-api
description: Add or change flight-tracker HTTP endpoints, Chi routes, query parsing, JSON handlers, error bodies, and swag OpenAPI docs. Use when working on internal/api, routes, handlers, query params, Swagger annotations, or REST responses.
---

# HTTP API

Also follow [development-practices](../development-practices/SKILL.md). After annotation or route changes, run `make swagger` and commit `docs/`.

## Layers

1. **Route** — register in `internal/api/server.go` under `/api/v1` (or top-level for health).
2. **Parse** — query strings in `internal/api/query` (`query` struct tags + `BindQuery` + `Validate`). JSON bodies on `internal/model` with `Validate()`.
3. **Handler** — thin: parse → store call → `writeJSON`. No SQL, no remote downloads.
4. **Swagger** — comment annotations on the handler. Visibility is the swag tag list.

Constructor wiring: add the handler in `newRouter` (`internal/api/server.go`). If it needs a new collaborator, thread it from `cmd/server` → `api.NewServer` → `newRouter`.

## Query parsing

Copy `internal/api/query/routes.go` / `carriers.go` / `jobs.go`:

- Struct tags: `` `query:"name" validate:"..."` ``
- `BindQuery` then `normalizeQueryStrings` (uppercases `origin`, `dest`, `carrier`, `state`) then `Validate`.
- Custom messages belong in `formatValidationFieldError` (`internal/api/query/validate.go`).
- Dates are `time.Time` decoded as `YYYY-MM-DD`.
- Comma-separated `days_of_week` is expanded in `BindQuery` — add other comma lists there if needed.
- Handlers map parse errors to **400** with `ErrorResponse{Error: err.Error()}`.

## JSON bodies

Decode with `json.NewDecoder(r.Body).Decode`. Invalid JSON → **400** `invalid json body` (`errInvalidJSONBody`). Empty optional bodies (reference ingest) have an existing decode helper — reuse it.

## Status codes

| Code | When |
| --- | --- |
| 200 | Successful GET |
| 201 | Jobs queued |
| 400 | Bind/validation/range errors |
| 404 | `errors.Is(err, store.ErrNotFound)` |
| 409 | Active ingest job or existing data without `force` |
| 500 | Store/internal failures with a stable message |
| 503 | Readiness ping failure |

**500** messages must be generic (`failed to compute route stats`). Conflict bodies use the typed structs in `internal/api/handlers/swagger_models.go` (`active_ingest_months`, `existing_data_months`, `job_type`, `dataset`).

Always `writeJSON` (`internal/api/handlers/health.go`). Do not invent a second encoder.

## Swagger

Two generated specs from `cmd/server/main.go` `go:generate`:

- `@Tags foo,external` — user-facing `/swagger/` (`docs/external`). Today: route stats, route outlook, carrier stats.
- `@Tags foo,internal` — operator `/swagger/internal/` (`docs/full`). Health, ingest, jobs.

Include both the domain tag (`routes`, `ingest`, `jobs`, …) and `external` or `internal`. New user-facing reads should be `external`; ingest/ops remain `internal`.

Annotate every exported handler. Response types must be exported so swag can see them. Reuse `ErrorResponse` and the conflict structs.

Never edit files under `docs/` except via `make swagger`. Pin stays `swag` `v1.16.6`.

Internal UI is registered **before** `/swagger/*` in the router.

## Tests

Add handler tests in `internal/api/handlers/*_test.go` with `storetest.Stub`. Cover 400 validation, 409 conflicts, 404, and the 201/200 happy path. Query-only logic gets tests in `internal/api/query`.

Assert the current response contract. Do not add tests whose purpose is to prove a removed field or behavior is gone (see [testing.md](../development-practices/testing.md)).
