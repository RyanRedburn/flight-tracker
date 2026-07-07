# flight-tracker

![Lint](https://github.com/RyanRedburn/flight-tracker/actions/workflows/lint.yml/badge.svg?branch=main)
![Test](https://github.com/RyanRedburn/flight-tracker/actions/workflows/test.yml/badge.svg?branch=main)

Go service with a REST API and an in-process background worker for importing BTS on-time flight data.

## Features

- `POST /api/v1/ingest` to queue per-month BTS import jobs
- Poll-based background workers that download, parse, and load flight data into SQLite
- REST API to query on-time flights and job status
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
| `MAX_INGEST_MONTHS` | `24` | Max months per ingest request |
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

# Get job status
curl http://localhost:8080/api/v1/jobs/<job-id>

# List recent jobs
curl http://localhost:8080/api/v1/jobs

# Query on-time flights (optional filters: flight_date, origin, dest, limit, offset; default limit 50, max 500)
curl "http://localhost:8080/api/v1/flights?flight_date=2026-04-24&origin=ORD&dest=BHM&limit=10"
```

### Ingest behavior

- Creates one `import_bts_on_time` job per month in the requested range.
- Omit `end_year` and `end_month` to ingest a single month (`start_year` / `start_month`).
- `start_year` must be >= 2018 (earliest BTS on-time data in this service).
- Workers poll the database, download the BTS zip for each month, and load `on_time_flights`.
- Returns **409** if a pending/running ingest job already exists for a requested month.
- Returns **409** if flight data already exists and `force` is not set.
- `force: true` skips the data-exists check; workers always replace the target month on import.
- Requested ranges are capped by `MAX_INGEST_MONTHS` (default 24).

`internal/ingest/bts/testdata/` contains a small BTS CSV sample (header plus 20 diverse April 2026 rows) used by parser and ingest tests. It is not used in production — imports come from TranStats at runtime.

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
internal/api/         HTTP server, handlers, middleware, query parsing
internal/config/      Environment configuration
internal/database/    Store factory (driver selection)
internal/ingest/      Ingest range expansion; BTS download/parse/load (ingest/bts/)
internal/model/       Domain types
internal/operator/    Background worker and job processor
internal/store/       Store interface, queries, SQLite + in-memory implementations
internal/validation/  Shared request/query validation
docker/migrate/       Migrate sidecar (Dockerfile + Makefile for up/down/shell)
migrations/           Per-driver SQL migrations (sqlite/, postgres/)
```
