# flight-tracker

![Lint](https://github.com/RyanRedburn/flight-tracker/actions/workflows/lint.yml/badge.svg?branch=main)
![Test](https://github.com/RyanRedburn/flight-tracker/actions/workflows/test.yml/badge.svg?branch=main)

Go service with a REST API and an in-process background worker for importing BTS on-time flight data and OurAirports reference data (countries, regions, airports).

## Features

- `POST /api/v1/ingest` to queue per-month BTS import jobs
- `POST /api/v1/ingest/countries`, `/regions`, and `/airports` to queue OurAirports reference data imports
- Poll-based background workers that download, parse, and load data into SQLite
- REST API for route performance stats, booking outlook probabilities, and job status
- Per-driver SQL migrations via [golang-migrate](https://github.com/golang-migrate/migrate)
- Docker deployment with persistent SQLite volume

## Requirements

- Go 1.25+
- [GNU Make](https://www.gnu.org/software/make/) (Git Bash, WSL, or `choco install make` on Windows)
- [golangci-lint](https://golangci-lint.run/) v2 (for `make lint`)
- C compiler (CGO) for local SQLite builds and the full test suite, **or** use Docker
- Docker & Docker Compose (optional)

## Makefile

| Command | Description |
| --------- | ------------- |
| `make lint` | Run golangci-lint (uses `.golangci.yml`) |
| `make swagger` | Regenerate OpenAPI docs (`docs/external`, `docs/full`) via `go generate` |
| `make test` | Unit tests only — no C compiler required |
| `make test-all` | Full suite including SQLite integration tests (requires CGO/gcc) |
| `make test-cover-html` | Full suite with HTML coverage report (`coverage.html`) |
| `make docker-build` | Build Docker images |
| `make docker-run` | Start the app via Docker Compose |
| `make migrate-up` | Apply migrations via the migrate sidecar |
| `make migrate-down` | Roll back one migration |
| `make migrate-version` | Show current migration version |
| `make db-shell` | Interactive SQLite shell against the Docker volume |
| `make clean-cover` | Remove generated coverage files |

## Local development

```bash
# Requires CGO (gcc). On Windows without a C compiler, use Docker instead.
CGO_ENABLED=1 go run ./cmd/server
```

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

After editing annotations, re-run `make swagger` (or `go generate ./cmd/server/...`) and commit the updated files under `docs/`.

### Tests

```bash
make test       # unit tests (no C compiler)
make test-all   # full suite including SQLite (requires CGO)
```

Environment variables (defaults shown):

| Variable | Default | Description |
| ---------- | --------- | ------------- |
| `HTTP_ADDR` | `:8080` | API listen address |
| `DATABASE_DRIVER` | `sqlite` | `sqlite` or `postgres` |
| `DATABASE_URL` | `file:flight-tracker.db` | Database DSN |
| `MIGRATIONS_PATH` | `migrations/sqlite` | Migration folder for the active driver |
| `WORKER_CONCURRENCY` | `2` | Background worker goroutines |
| `WORKER_POLL_INTERVAL` | `5s` | How often workers poll for pending jobs |
| `STALE_JOB_THRESHOLD` | `30m` | Reset stuck `running` jobs on startup |
| `BTS_DOWNLOAD_TIMEOUT` | `10m` | HTTP timeout for BTS zip downloads |
| `BTS_BASE_URL` | `https://transtats.bts.gov/PREZIP` | BTS zip base URL (override in tests) |
| `OURAIRPORTS_BASE_URL` | `https://raw.githubusercontent.com/davidmegginson/ourairports-data/main` | OurAirports CSV base URL |
| `OURAIRPORTS_DOWNLOAD_TIMEOUT` | `5m` | HTTP timeout for OurAirports CSV downloads |
| `MAX_INGEST_MONTHS` | `24` | Max months per BTS ingest request |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Docker

Compose defines two services: `app` (distroless API server) and `migrate` (migration CLI and SQLite shell). The app image has no shell or extra tools — use the migrate sidecar for manual migrations and database inspection.

```bash
make docker-build          # build images
make docker-run            # start the app (foreground)
docker compose up --build  # build and start in one step
```

The app listens on port 8080 and stores SQLite data in the `flight-data` volume at `/data/flight-tracker.db`.

## API examples

```bash
# Liveness
curl http://localhost:8080/health

# Readiness (database ping)
curl http://localhost:8080/ready

# Database migration version
curl http://localhost:8080/db/version

# Queue BTS on-time data import for a single month
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

# Queue OurAirports reference data imports (recommended order: countries → regions → airports)
curl -X POST http://localhost:8080/api/v1/ingest/countries \
  -H "Content-Type: application/json" \
  -d '{}'

curl -X POST http://localhost:8080/api/v1/ingest/regions \
  -H "Content-Type: application/json" \
  -d '{}'

curl -X POST http://localhost:8080/api/v1/ingest/airports \
  -H "Content-Type: application/json" \
  -d '{}'

# Re-import OurAirports data that already exists
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

#### BTS on-time flights (`POST /api/v1/ingest`)

- Creates one `import_bts_on_time` job per month in the requested range.
- Omit `end_year` and `end_month` to ingest a single month (`start_year` / `start_month`).
- `start_year` must be >= 2018 (earliest BTS on-time data in this service).
- Workers poll the database, download the BTS zip for each month, and load `on_time_flights`.
- Returns **409** if a pending/running ingest job already exists for a requested month.
- Returns **409** if flight data already exists and `force` is not set.
- `force: true` skips the data-exists check; workers always replace the target month on import.
- Requested ranges are capped by `MAX_INGEST_MONTHS` (default 24).

`internal/ingest/bts/testdata/` contains a small BTS CSV sample (header plus 20 diverse April 2026 rows) used by parser and ingest tests. It is not used in production — imports come from TranStats at runtime.

#### OurAirports reference data (`POST /api/v1/ingest/{countries|regions|airports}`)

- Creates one job per request (`import_ourairports_countries`, `import_ourairports_regions`, or `import_ourairports_airports`).
- Workers download the CSV from the OurAirports GitHub data dump and full-replace the matching table.
- Returns **409** if a pending/running job already exists for that dataset.
- Returns **409** if the table already has rows and `force` is not set.
- `force: true` skips the data-exists check; workers always replace the full table on import.
- Recommended load order: **countries → regions → airports** (no FK constraints; order is for data consistency only).
- Source: [OurAirports open data](https://ourairports.com/data/) (public domain), nightly dumps on [davidmegginson/ourairports-data](https://github.com/davidmegginson/ourairports-data).

## Migrations

Migrations run automatically on server startup.

### Via Docker (recommended)

Uses the shared `flight-data` volume — the same database file the running app uses.

```bash
make migrate-up
make migrate-version
make db-shell
```

Inside the SQLite shell, column headers are off by default. Turn them on with `.headers on` and `.mode column`.

One-off query without opening a shell:

```bash
docker compose run --rm --entrypoint sqlite3 migrate /data/flight-tracker.db "SELECT * FROM jobs;"
```

### Local CLI

Install the migrate CLI:

```bash
go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Then run manually:

```bash
migrate -path migrations/sqlite -database "sqlite3://./flight-tracker.db" up
```

## Project layout

```text
cmd/server/           Entry point
docs/external/        Generated OpenAPI (user-facing / external Swagger)
docs/full/            Generated OpenAPI (full / internal Swagger)
internal/api/         HTTP server, handlers, middleware, query parsing
internal/config/      Environment configuration
internal/database/    Store factory (driver selection)
internal/ingest/      Ingest range expansion; BTS and OurAirports download/parse/load
internal/model/       Domain types
internal/operator/    Background worker and job processor
internal/store/       Store interface, queries, SQLite + in-memory implementations
docker/migrate/       Migrate sidecar (Dockerfile + Makefile for up/down/shell)
migrations/           Per-driver SQL migrations (sqlite/, postgres/)
```
