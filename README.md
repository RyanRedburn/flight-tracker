# flight-tracker

![Lint](https://github.com/RyanRedburn/flight-tracker/actions/workflows/lint.yml/badge.svg?branch=main)
![Test](https://github.com/RyanRedburn/flight-tracker/actions/workflows/test.yml/badge.svg?branch=main)
![Swagger](https://github.com/RyanRedburn/flight-tracker/actions/workflows/swagger.yml/badge.svg?branch=main)

Go service with a REST API and an in-process background worker for importing flight performance data and airport reference data (countries, regions, airports).

## Features

- `POST /api/v1/ingest` to queue per-month flight performance import jobs
- `POST /api/v1/ingest/countries`, `/regions`, and `/airports` to queue reference data imports
- Poll-based background workers that download, parse, and load data into Postgres
- REST API for route performance stats, booking outlook probabilities, and job status
- SQL migrations via [golang-migrate](https://github.com/golang-migrate/migrate)
- Docker Compose with Postgres and a migrate sidecar

## Requirements

- Go 1.25+
- [GNU Make](https://www.gnu.org/software/make/) (Git Bash, WSL, or `choco install make` on Windows)
- [golangci-lint](https://golangci-lint.run/) v2 (for `make lint`)
- Docker & Docker Compose (for Postgres and optional containerized runs)

## Makefile

| Command | Description |
| --------- | ------------- |
| `make lint` | Run golangci-lint (uses `.golangci.yml`) |
| `make swagger` | Regenerate OpenAPI docs (`docs/external`, `docs/full`) via `go generate` |
| `make test` | Run all tests |
| `make test-cover-html` | Full suite with HTML coverage report (`coverage.html`) |
| `make docker-build` | Build Docker images |
| `make docker-run` | Start the app via Docker Compose |
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

Swagger UI (after the server is running):

- External (user-facing): [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- Internal (full API): [http://localhost:8080/swagger/internal/index.html](http://localhost:8080/swagger/internal/index.html)

Regenerate docs after changing swag annotations (uses pinned `swag` `v1.16.6` via `go run`, no global install required):

```bash
make swagger
```

Visibility is controlled by swag tags on each handler:

- `external` — included in the user-facing `/swagger/` docs (currently route stats and outlook only)
- `internal` — operator/admin endpoints; appear only under `/swagger/internal/`

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
| `STALE_JOB_THRESHOLD` | `30m` | Reset stuck `running` jobs on startup |
| `BTS_DOWNLOAD_TIMEOUT` | `10m` | HTTP timeout for BTS (flight performance source) zip downloads |
| `BTS_BASE_URL` | `https://transtats.bts.gov/PREZIP` | BTS zip base URL (override in tests) |
| `OURAIRPORTS_BASE_URL` | `https://raw.githubusercontent.com/davidmegginson/ourairports-data/main` | OurAirports (reference data source) CSV base URL |
| `OURAIRPORTS_DOWNLOAD_TIMEOUT` | `5m` | HTTP timeout for OurAirports CSV downloads |
| `MAX_INGEST_MONTHS` | `24` | Max months per flight-performance ingest request |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Docker

Compose defines three services: `postgres`, `app` (distroless API server), and `migrate` (migration CLI and `psql`). The app image has no shell or extra tools — use the migrate sidecar for manual migrations and database inspection.

Optional local overrides: copy [`.env.example`](.env.example) to `.env` (Compose defaults match the example credentials).

```bash
make docker-build          # build images
docker compose up -d postgres   # start Postgres only
make docker-run            # start the stack (foreground)
docker compose up --build  # build and start in one step
```

The app listens on port 8080. Postgres data is stored in the `postgres-data` volume. Postgres is published on host port `5432` for local tooling.

## API examples

```bash
# Liveness
curl http://localhost:8080/health

# Readiness (database ping)
curl http://localhost:8080/ready

# Database migration version
curl http://localhost:8080/db/version

# Queue flight performance data import for a single month
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"start_year":2026,"start_month":4}'

# Queue import for a month range
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"start_year":2026,"start_month":1,"end_year":2026,"end_month":4}'

# Re-import months that already have data
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"start_year":2026,"start_month":4,"force":true}'

# Queue reference data imports (recommended order: countries → regions → airports)
curl -X POST http://localhost:8080/api/v1/ingest/countries \
  -H "Content-Type: application/json" \
  -d '{}'

curl -X POST http://localhost:8080/api/v1/ingest/regions \
  -H "Content-Type: application/json" \
  -d '{}'

curl -X POST http://localhost:8080/api/v1/ingest/airports \
  -H "Content-Type: application/json" \
  -d '{}'

# Re-import reference data that already exists
curl -X POST http://localhost:8080/api/v1/ingest/airports \
  -H "Content-Type: application/json" \
  -d '{"force":true}'

# Get job status
curl http://localhost:8080/api/v1/jobs/<job-id>

# List recent jobs
curl http://localhost:8080/api/v1/jobs

# Route performance stats for a date range (required: origin, dest, start_date, end_date;
# optional: carrier, flight_number [requires carrier], days_of_week=1-7 Mon-Sun; max span 366 days)
curl "http://localhost:8080/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2025-01-01&end_date=2025-06-30&carrier=UA&days_of_week=1,2,3,4,5"

# Booking outlook probabilities for a departure slot (required: origin, dest, carrier, day_of_week, dep_time;
# optional: dep_time_window_minutes, default 30, circular around midnight; uses last 365 days of matching history)
curl "http://localhost:8080/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700"
```

### Ingest behavior

#### Flight performance (`POST /api/v1/ingest`)

- Creates one `import_flight_performance` job per month in the requested range.
- Omit `end_year` and `end_month` to ingest a single month (`start_year` / `start_month`).
- `start_year` must be >= 2018 (earliest flight performance data supported by this service).
- Workers poll the database, download the source zip for each month, and load `flight_performance`.
- Returns **409** if a pending/running ingest job already exists for a requested month.
- Returns **409** if flight data already exists and `force` is not set.
- `force: true` skips the data-exists check; workers always replace the target month on import.
- Requested ranges are capped by `MAX_INGEST_MONTHS` (default 24).

Source adapter: BTS TranStats Marketing Carrier On-Time Performance. `internal/ingest/bts/testdata/` contains a small CSV sample (header plus 20 diverse April 2026 rows) used by parser and ingest tests. It is not used in production — imports come from TranStats at runtime.

#### Reference data (`POST /api/v1/ingest/{countries|regions|airports}`)

- Creates one job per request (`import_countries`, `import_regions`, or `import_airports`).
- Workers download the CSV and full-replace the matching table.
- Returns **409** if a pending/running job already exists for that dataset.
- Returns **409** if the table already has rows and `force` is not set.
- `force: true` skips the data-exists check; workers always replace the full table on import.
- Recommended load order: **countries → regions → airports** (no FK constraints; order is for data consistency only).

Source adapter: [OurAirports open data](https://ourairports.com/data/) (public domain), nightly dumps on [davidmegginson/ourairports-data](https://github.com/davidmegginson/ourairports-data).

## Migrations

Migrations run automatically on server startup (against `MIGRATIONS_PATH`).

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
internal/ingest/      Ingest range expansion; provider adapters (BTS, OurAirports) download/parse/load
internal/model/       Domain types
internal/operator/    Background worker and job processor
internal/store/       Store interface, queries, Postgres implementation, test stub
docker/migrate/       Migrate sidecar (Dockerfile + Makefile for up/down/psql)
migrations/           SQL migrations (postgres/)
```
