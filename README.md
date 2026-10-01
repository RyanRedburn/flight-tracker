# flight-tracker

![Lint](https://github.com/RyanRedburn/flight-tracker/actions/workflows/lint.yml/badge.svg?branch=main)
![Test](https://github.com/RyanRedburn/flight-tracker/actions/workflows/test.yml/badge.svg?branch=main)
![Swagger](https://github.com/RyanRedburn/flight-tracker/actions/workflows/swagger.yml/badge.svg?branch=main)

Go HTTP API and in-process workers. Workers import BTS on-time performance, IEM airport weather, OurAirports reference data, and airport minimum connection times into Postgres. Reads cover route and carrier stats, typical-year travel windows, weather-category stats, and booking outlook for one route or a short itinerary.

## Features

Request and response contracts live in Swagger (see [Local development](#local-development)): user-facing stats and outlook, plus admin ingest, job status, data freshness, travel-window and weather-stats rebuilds, and API keys.

- Poll-based workers download, parse, and load Postgres
- Hashed API keys in Postgres (`consumer`, `subscriber`, `admin`) with shared, multi-replica rate limits
- SQL migrations via [golang-migrate](https://github.com/golang-migrate/migrate)
- Docker Compose with Postgres and a migrate sidecar

## Requirements

- Go 1.25+
- [GNU Make](https://www.gnu.org/software/make/) (Git Bash, WSL, or `choco install make` on Windows)
- Docker & Docker Compose (for Postgres and optional containerized runs)

## Makefile

| Command | Description |
| --------- | ------------- |
| `make lint` | Run pinned golangci-lint `v2.12.2` via `go run` (`.golangci.yml`; no global install) |
| `make swagger` | Regenerate OpenAPI docs (`docs/external`, `docs/full`) via `go generate` |
| `make test` | Run all tests |
| `make test-cover-html` | Full suite with HTML coverage report (`coverage.html`) |
| `make docker-build` | Build Docker images |
| `make docker-run` | Start postgres and the app via Docker Compose (migrate sidecar is not part of default `up`) |
| `make migrate-up` | Apply migrations via the migrate sidecar |
| `make migrate-down` | Roll back one migration |
| `make migrate-version` | Show current migration version |
| `make db-shell` | Interactive `psql` against Compose Postgres |
| `make clean-cover` | Remove generated coverage files |

## Local development

Start Postgres (Compose), then run the server locally:

```bash
docker compose up -d postgres
go run ./cmd/server
```

Defaults expect Postgres at `localhost:5432` with the credentials in [`.env.example`](.env.example). Or run the full stack with `make docker-run`.

Local `go run` does not load `.env`. Auth is **on** by default (`AUTH_DISABLED=false`). For unauthenticated local use:

```bash
AUTH_DISABLED=true go run ./cmd/server
```

Compose defaults `AUTH_DISABLED=true` so `make docker-run` stays usable without keys. Do not ship that value to production.

Swagger UI (after the server is running):

- External (user-facing): [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- Internal (full API): [http://localhost:8080/swagger/internal/index.html](http://localhost:8080/swagger/internal/index.html)

Regenerate docs after changing swag annotations (uses pinned `swag` `v1.16.6` via `go run`, no global install required):

```bash
make swagger
```

Visibility is controlled by swag tags on each handler:

- `external` — user-facing `/swagger/` (route and carrier stats, travel windows, weather stats, route outlook, itinerary outlook)
- `internal` — operator/admin endpoints; appear only under `/swagger/internal/` (ingest, jobs, data freshness, rebuilds, keys)

When authentication is enabled, `/swagger/` requires a `consumer`, `subscriber`, or `admin` key, and `/swagger/internal/` requires `admin`. `/health` and `/ready` stay unauthenticated.

After editing annotations, re-run `make swagger` (or `go generate ./cmd/server/...`) and commit the updated files under `docs/`. CI runs the same regenerate step and fails if `docs/` drifts.

### Tests

```bash
make test
```

Environment variables (defaults shown):

| Variable | Default | Description |
| ---------- | --------- | ------------- |
| `HTTP_ADDR` | `:8080` | API listen address |
| `DATABASE_DRIVER` | `postgres` | Database driver (`postgres` only) |
| `DATABASE_URL` | `postgres://flight:flight@localhost:5432/flight_tracker?sslmode=disable` | Database DSN |
| `MIGRATIONS_PATH` | `migrations/postgres` | Migration folder |
| `WORKER_CONCURRENCY` | `2` | Background worker goroutines |
| `WORKER_POLL_INTERVAL` | `5s` | How often workers poll for pending jobs |
| `JOB_LEASE_TTL` | `90s` | How long a running job's lease lasts without a heartbeat. Expired leases are requeued to `pending` (crash/OOM/SIGKILL). `0` disables reclaim. Heartbeats run about every TTL/3 while `Process` is in flight. |
| `BTS_DOWNLOAD_TIMEOUT` | `10m` | HTTP timeout for BTS (flight performance source) zip downloads |
| `BTS_BASE_URL` | `https://transtats.bts.gov/PREZIP` | BTS zip base URL (override in tests) |
| `IEM_ASOS_BASE_URL` | `https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py` | IEM ASOS CGI endpoint (override in tests) |
| `IEM_ASOS_DOWNLOAD_TIMEOUT` | `10m` | HTTP timeout for IEM ASOS CSV downloads |
| `IEM_GEOJSON_BASE_URL` | `https://mesonet.agron.iastate.edu/geojson/network` | IEM network GeoJSON base for station metadata |
| `IEM_GEOJSON_TIMEOUT` | `2m` | HTTP timeout for IEM station GeoJSON downloads |
| `OURAIRPORTS_BASE_URL` | `https://raw.githubusercontent.com/davidmegginson/ourairports-data/main` | OurAirports (reference data source) CSV base URL |
| `OURAIRPORTS_DOWNLOAD_TIMEOUT` | `5m` | HTTP timeout for OurAirports CSV downloads |
| `MCT_BASE_URL` | `https://minimumconnectiontime.com` | Minimum Connection Time API origin (override in tests) |
| `MCT_HTTP_TIMEOUT` | `2m` | Per-request HTTP timeout for paginated airport MCT downloads |
| `MAX_INGEST_MONTHS` | `24` | Max months per flight-performance ingest request |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `AUTH_DISABLED` | `false` | When `true`, skip API authentication **and** rate limits. Explicit local/dev fail-open; the process logs a warning. Production must leave this false. |
| `RATE_LIMIT_DISABLED` | `false` | When `true`, skip rate limits but still authenticate. `AUTH_DISABLED=true` also skips rate limits. |
| `RATE_LIMIT_TRUST_PROXY` | `false` | When `true`, anonymous limits use the right-most `X-Forwarded-For` hop (then `X-Real-IP`, then `RemoteAddr`). Enable only behind a single trusted reverse proxy. There is no hop-count stripping of extra forwarded addresses. |
| `AUTH_BOOTSTRAP_ADMIN_KEY` | empty | One-time admin key inserted if `api_keys` is empty. Must be `ftk_<8 hex>_<32 hex>`. Unset after first boot. Concurrent replicas treat a unique conflict as success. |
| `RATE_LIMIT_ANON_RPM` | `60` | Shared requests/minute for true anonymous public routes (`ip|<ip>|<surface>`). Probes are not limited. Failed credentials do **not** use this bucket. |
| `RATE_LIMIT_CONSUMER_RPM` | `120` | Shared requests/minute for `consumer` keys. |
| `RATE_LIMIT_SUBSCRIBER_RPM` | `120` | Shared requests/minute for `subscriber` keys (same access as `consumer` in v1). |
| `RATE_LIMIT_ADMIN_RPM` | `300` | Shared requests/minute for `admin` keys on non-ingest surfaces. |
| `RATE_LIMIT_ADMIN_INGEST_RPM` | `10` | Shared requests/minute for `admin` ingest POSTs so a loop cannot flood the job queue. |
| `RATE_LIMIT_AUTH_FAIL_RPM` | `30` | Shared requests/minute for missing, invalid, or revoked credentials on protected routes (`ip|<ip>|auth_fail`). Keeps auth spam from exhausting the anonymous public IP quota. |

### Authentication and rate limiting

API replicas share one Postgres. Auth and advertised rate limits are **not** per-process.

**Credentials.** Send the plaintext key as `Authorization: Bearer <key>` (primary) or `X-API-Key: <key>`. If both are present they must match. Keys are stored as SHA-256 hashes keyed by an 8-character prefix (`ftk_<prefix>_<secret>`); the plaintext is returned only at creation. Lookup is by prefix, then a constant-time hash compare.

**Roles.**

| Role | Access |
| --- | --- |
| `consumer` | External API (`/api/v1/routes/*`, `/api/v1/itineraries/*`, `/api/v1/carriers/*`) and `/swagger/` |
| `subscriber` | Same allow-list as `consumer` in v1 (role is stored distinctly for later use) |
| `admin` | Everything: ingest, jobs, data freshness, `/db/version`, `/swagger/internal/`, key management, and consumer surfaces |

`/health` and `/ready` are unauthenticated and are not rate-limited. Ingest is never anonymous — **admin only**, with `RATE_LIMIT_ADMIN_INGEST_RPM`. Protected routes fail closed when auth is enabled (missing/invalid/revoked key → **401**, or **429** if the auth-fail IP bucket is exhausted; wrong role → **403**).

**Bootstrap.** Auth enabled with an empty `api_keys` table refuses to start (not fail-open). Either:

1. Set `AUTH_BOOTSTRAP_ADMIN_KEY` to a generated `ftk_…` value for the first process start, then unset it, or
2. Run once with `AUTH_DISABLED=true`, `POST /api/v1/keys` as below, then restart with auth enabled.

```bash
python3 -c "import secrets; print('ftk_' + secrets.token_hex(4) + '_' + secrets.token_hex(16))"
```

**Rate limits.** Postgres holds a token bucket per identity × surface. Capacity and refill equal the configured requests/minute, so N replicas cannot multiply the advertised cap. A small per-process burst shield (200 rps) only sheds floods before they hit Postgres; it is not the advertised quota.

| Bucket | When |
| --- | --- |
| `key|<id>|<surface>` | Valid API key (`external`, `internal`, or `ingest`) |
| `ip|<client ip>|auth_fail` | Missing, invalid, or revoked credentials on a **protected** route (`RATE_LIMIT_AUTH_FAIL_RPM`) |
| `ip|<client ip>|<surface>` | True anonymous public routes (none in v1 besides unlimited probes). Not used for failed credentials. |

Shared-store (Postgres) responses include:

| Header | Meaning |
| --- | --- |
| `X-RateLimit-Limit` | Configured requests/minute for this identity and surface |
| `X-RateLimit-Remaining` | Whole tokens left (0 on **429**) |
| `X-RateLimit-Reset` | Unix timestamp (UTC seconds) when the bucket is projected to be full, or when the next request is allowed on **429** |
| `Retry-After` | Seconds until the next token (**429** only) |

A process-local burst-shield **429** includes `Retry-After` only. It does not set `X-RateLimit-*`; those headers always reflect the shared Postgres bucket.

**Client IP.** Default is `RemoteAddr` (correct for Compose port publish / a directly reached process). `RATE_LIMIT_TRUST_PROXY=true` uses the right-most `X-Forwarded-For` address. Do not enable that on an untrusted network; spoofed extra hops are not stripped.

## Docker

Compose defines three services: `postgres`, `app` (distroless API server), and `migrate` (migration CLI and `psql`). The app image is `gcr.io/distroless/static-debian12:nonroot` and runs as UID/GID 65532. It reads `/server` and `/migrations` (startup migrations still run inside the app and write only to Postgres) and listens on 8080. Default bring-up (`make docker-run` / `docker compose up`) starts postgres and app only — the app migrates on startup. The migrate sidecar uses the `migrate` profile so it is not started alongside the app (that would double-migrate on first boot). The app image has no shell or extra tools; use the sidecar for manual migrations and database inspection (`make migrate-up`, `make db-shell`).

Optional local overrides: copy [`.env.example`](.env.example) to `.env` (Compose defaults match the example credentials).

```bash
make docker-build          # build images
docker compose up -d postgres   # start Postgres only
make docker-run            # start the stack (foreground)
docker compose up --build  # build and start in one step
```

The app listens on port 8080. Postgres data is stored in the `postgres-data` volume. Postgres is published on host port `5432` for local tooling. Compose sends SIGTERM to the app with a **30s** `stop_grace_period` so workers can abort in-flight jobs and persist `failed` before the container is killed. That window is for HTTP drain plus status writes — not for finishing a multi-minute download.

If you run under Kubernetes, set `terminationGracePeriodSeconds` similarly (at least ~30s). Do not size it to cover a full BTS/IEM download; SIGTERM aborts work and marks the job failed.

## API examples

When `AUTH_DISABLED=true`, these curls work as written. With authentication on, send `-H "Authorization: Bearer $API_KEY"` on protected routes (`/health` and `/ready` stay open). A route or carrier with no flight-performance history is **404**. A filter that matches no rows still returns **200** with zeros or a zero sample. A valid itinerary body returns **200**, and a leg with no history has a null outlook. Field rules are in Swagger.

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/db/version

curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"start_year":2026,"start_month":4}'
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"start_year":2026,"start_month":1,"end_year":2026,"end_month":4,"force":true}'

curl -X POST http://localhost:8080/api/v1/ingest/countries -H "Content-Type: application/json" -d '{}'
curl -X POST http://localhost:8080/api/v1/ingest/regions -H "Content-Type: application/json" -d '{}'
curl -X POST http://localhost:8080/api/v1/ingest/airports -H "Content-Type: application/json" -d '{}'
curl -X POST http://localhost:8080/api/v1/ingest/mct -H "Content-Type: application/json" -d '{}'
curl -X POST http://localhost:8080/api/v1/ingest/weather-stations -H "Content-Type: application/json" -d '{}'
curl -X POST http://localhost:8080/api/v1/ingest/weather \
  -H "Content-Type: application/json" \
  -d '{"start_year":2024,"start_month":1}'
curl -X POST http://localhost:8080/api/v1/ingest/weather \
  -H "Content-Type: application/json" \
  -d '{"start_year":2024,"start_month":1,"stations":["ORD","JFK","ATL"]}'

curl http://localhost:8080/api/v1/jobs/<job-id>
curl http://localhost:8080/api/v1/jobs
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/data-freshness

curl "http://localhost:8080/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2025-01-01&end_date=2025-06-30&days_of_week=1,2,3,4,5"
curl "http://localhost:8080/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700"
curl -X POST http://localhost:8080/api/v1/itineraries/outlook \
  -H "Content-Type: application/json" \
  -d '{"legs":[{"origin":"BOS","dest":"ORD","carrier":"UA","date":"2026-10-06","dep_time":"0700","arr_time":"0905"},{"origin":"ORD","dest":"LAX","carrier":"UA","date":"2026-10-06","dep_time":"1100","arr_time":"1330"}]}'
curl "http://localhost:8080/api/v1/routes/travel-windows?origin=ORD&dest=LAX"
# Categories and the ±30 minute join: internal/ingest/iem/documents/asos_observations.md
curl "http://localhost:8080/api/v1/routes/weather-stats?origin=ORD&dest=LAX&start_date=2025-01-01&end_date=2025-06-30"
curl -X POST -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/rebuild/travel-windows
curl -X POST -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/rebuild/weather-stats
curl "http://localhost:8080/api/v1/carriers/stats?carrier=UA&state=IL"

curl -X POST http://localhost:8080/api/v1/keys \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"role":"consumer","name":"docs example"}'
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/keys
curl -X POST -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/keys/<key-id>/revoke
```

### Ingest behavior

Month-range imports (flight performance and weather observations) create one job per month. Omit `end_year` and `end_month` for a single month. `start_year` must be >= 2018. A range longer than `MAX_INGEST_MONTHS` (default 24) is rejected.

Every ingest returns **409** when a pending or running job already covers that month or dataset, including when `force` is true. It also returns **409** when the target already has rows and `force` is omitted. `force: true` skips only the exists check. Workers still replace the target month or full table.

Load order for a consistent database (there are no foreign keys): **countries → regions → airports → at least one BTS month → weather-stations → weather observations**. Airport MCT does not depend on that order.

A successful flight-performance load rebuilds travel-window rollups and weather-category rollups. A successful weather-observation or weather-station load rebuilds weather-category rollups only. Admins can queue the same full rebuilds with an empty body: `POST /api/v1/rebuild/travel-windows` and `POST /api/v1/rebuild/weather-stats`. Each returns **409** when that rebuild type is already pending or running. Category names and the ±30 minute join are in `internal/ingest/iem/documents/asos_observations.md`.

#### Flight performance (`POST /api/v1/ingest`)

Workers download the BTS TranStats Marketing Carrier On-Time Performance zip for the month and load `flight_performance`.

#### Weather observations (`POST /api/v1/ingest/weather`)

`stations` is optional. When omitted, IEM site ids come from `airport_weather_stations` where `matched = true` (ingest weather-stations first). An explicit list overrides that and is uppercased. The response may include `unresolved_airports` for mapping rows with `matched = false`. IEM downloads run at about one request per second and retry HTTP 503. Each stored row gets `ceiling_ft` and `category` at ingest.

Source: Iowa Environmental Mesonet ASOS/METAR (`asos.py`).

#### Weather stations (`POST /api/v1/ingest/weather-stations`)

One job downloads US IEM ASOS GeoJSON, replaces `weather_stations`, and rebuilds `airport_weather_stations` from distinct BTS origin and dest codes. Matching tries IEM `sid` against the BTS/IATA code, then OurAirports `local_code` (FAA), then `icao_code` / `ident`. Missing airports data falls back to exact IATA=`sid`. Unmatched airports are stored with `matched = false`. Empty BTS data still replaces the catalog and leaves the mapping empty. The weather-stats rebuild runs because station ids and timezones change which observation matches a flight.

#### Reference data (`POST /api/v1/ingest/{countries|regions|airports}`)

One job per request (`import_countries`, `import_regions`, or `import_airports`) downloads the CSV and replaces that table.

Source: [OurAirports open data](https://ourairports.com/data/) (public domain), nightly dumps on [davidmegginson/ourairports-data](https://github.com/davidmegginson/ourairports-data).

#### Airport minimum connection times (`POST /api/v1/ingest/mct`)

One `import_airport_mct` job pages `GET /api/airports` on [Minimum Connection Time](https://minimumconnectiontime.com) at about one request per second, backing off on HTTP 429 and 5xx. It does not send `minimal=true`, because that view omits the minute fields. HTTP handlers do not call this API. A successful job replaces `airport_mct`. A failed download leaves the table unchanged.

These minutes are planning estimates from public sources. They are not official OAG or IATA minimum connection times, and they are not airline-, terminal-, or flight-number-specific. Link [minimumconnectiontime.com](https://minimumconnectiontime.com) when a product publishes them. Re-import about once a month. `MCT_BASE_URL` and `MCT_HTTP_TIMEOUT` override the origin and the per-request timeout.

### Job leases and shutdown

Multiple app replicas share one Postgres. Workers claim with `FOR UPDATE SKIP LOCKED` and write `lease_expires_at` (claim sets the first lease; a ticker refreshes it while `Process` runs). Heartbeats are independent of download/parse/COPY, which can block for minutes.

- **Crash / SIGKILL / OOM / missed heartbeat:** when `lease_expires_at` is in the past, any replica requeues the row to `pending` and clears `started_at`. Reclaim runs at startup (expired leases only — never all `running` rows) and periodically while the process is up. `JOB_LEASE_TTL` defaults to 90s, long enough to miss a few heartbeats, not as long as a 10-minute BTS download.
- **SIGINT / SIGTERM:** workers stop claiming immediately (they do not keep polling during HTTP drain), cancel in-flight `Process`, and persist `failed` with `interrupted by shutdown` using a context that is not cancelled. Idle workers exit. After shutdown-fail, re-POST the ingest (`force` if data already exists).
- Requeue after crash is safe: loads are transactional `Replace*` (full replace). Shutdown-fail is not an automatic retry.

## Migrations

Migrations run automatically on server startup (against `MIGRATIONS_PATH`). To set the recorded version without running SQL, use the migrate sidecar: `docker compose --profile migrate run --rm migrate force VERSION=N` (`make force` in that image).

### Via Docker (recommended)

Targets the Compose `postgres` service (same database the app uses).

```bash
docker compose up -d postgres
make migrate-up
make migrate-version
make db-shell
```

One-off query without opening a shell:

```bash
docker compose exec postgres psql -U flight -d flight_tracker -c "SELECT 1;"
```

### Local CLI

Install the migrate CLI:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Then run manually (with Compose Postgres running):

```bash
migrate -path migrations/postgres -database "postgres://flight:flight@localhost:5432/flight_tracker?sslmode=disable" up
```

## Project layout

```text
cmd/server/           Entry point
docs/external/        Generated OpenAPI (user-facing / external Swagger)
docs/full/            Generated OpenAPI (full / internal Swagger)
internal/api/         HTTP server, handlers, middleware, query parsing
internal/config/      Environment configuration
internal/database/    Store factory (driver selection)
internal/ingest/      Ingest range expansion; provider adapters (BTS, IEM, OurAirports, MCT) download/parse/load
internal/model/       Domain types
internal/operator/    Background worker and job processor
internal/store/       Store interface, queries, Postgres implementation, test stub
docker/migrate/       Migrate sidecar (Dockerfile + Makefile for up/down/force/psql)
migrations/           SQL migrations (postgres/)
```
