# flight-tracker

Go service with a REST API and an in-process background operator for fetching and processing flight data.

## Features

- REST API for triggering and querying async jobs
- Background worker pool processing jobs from a SQLite-backed queue
- Pluggable external data source interface (mock provider included)
- Per-driver SQL migrations via [golang-migrate](https://github.com/golang-migrate/migrate)
- Docker deployment with persistent SQLite volume

## Requirements

- Go 1.24+
- [GNU Make](https://www.gnu.org/software/make/) (Git Bash, WSL, or `choco install make` on Windows)
- [golangci-lint](https://golangci-lint.run/) (for `make lint`)
- C compiler (CGO) for local SQLite builds and the full test suite, **or** use Docker
- Docker & Docker Compose (optional)

## Makefile

| Command | Description |
|---------|-------------|
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

Equivalent raw commands:

```bash
# Unit tests (no CGO required)
go test ./internal/config/... ./internal/operator/... ./internal/api/... ./internal/source/... ./internal/store/mem/...

# Full suite including SQLite integration tests (requires CGO)
CGO_ENABLED=1 go test ./...
```

Environment variables (defaults shown):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | API listen address |
| `DATABASE_DRIVER` | `sqlite` | `sqlite` or `postgres` |
| `DATABASE_URL` | `file:flight-tracker.db` | Database DSN |
| `MIGRATIONS_PATH` | `migrations/sqlite` | Migration folder for the active driver |
| `WORKER_CONCURRENCY` | `2` | Background worker goroutines |
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

# Create a job
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"type":"fetch_flights","payload":{}}'

# Get job status
curl http://localhost:8080/api/v1/jobs/<job-id>

# List recent jobs
curl http://localhost:8080/api/v1/jobs
```

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
# SQLite
migrate -path migrations/sqlite -database "sqlite3://./flight-tracker.db" up

# Postgres (when implemented)
migrate -path migrations/postgres -database "postgres://..." up
```

## Postgres cutover (future)

1. Provision Cloud SQL Postgres
2. Run `migrations/postgres` against the instance
3. Implement `internal/store/postgres/`
4. Set `DATABASE_DRIVER=postgres` and `DATABASE_URL` in deployment
5. Optionally add Postgres to `docker-compose.yml` for local integration tests

## Project layout

```
cmd/server/          Entry point
internal/api/        HTTP server, handlers, middleware
internal/config/     Environment configuration
internal/model/      Domain types
internal/operator/   Background worker and job processor
internal/source/     External data provider interface
internal/database/   Store factory (driver selection)
internal/store/      Store interface, queries, SQLite + in-memory implementations
docker/migrate/      Migrate sidecar (Dockerfile + Makefile for up/down/shell)
migrations/          Per-driver SQL migrations (sqlite/, postgres/)
```

## Design notes

- **No ORMs** — persistence uses `database/sql`, sqlx, and hand-written SQL
- All database access goes through the `store.Store` interface
- UUIDs are generated in application code for portable primary keys
