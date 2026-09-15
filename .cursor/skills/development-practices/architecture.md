# Architecture

```text
HTTP client
    → internal/api (chi + handlers + query)
        → store.Store                    (queue jobs, read stats)
        → ingest helpers (range expand)

Worker goroutines (internal/operator)
    → ClaimNextPendingJob
    → JobHandler.Process
        → ingest/<provider>.Service     (download, parse, load)
            → store.Store               (Replace* / COPY)

cmd/server wires: config → database.NewStore → ingest services → operator.Processor/Worker → api.NewServer
```

## Request vs job

- **Read endpoints** (`/routes/*`, `/carriers/*`, `/jobs*`, health) call the store and return immediately.
- **Ingest POST** validates, checks active jobs / existing data, inserts `pending` jobs, returns **201**. Workers do the download.
- Do not download remote files or run multi-minute imports inside HTTP handlers.

## Ingest replace semantics

Workers always replace the target slice (month or full table). `force` only skips the HTTP-layer "data already exists" **409**. Pending/running jobs still **409**.

Recommended load order (no FKs; consistency only): countries → regions → airports → at least one BTS month → weather-stations → weather observations.

## Logging

Use `log/slog` with key/value pairs (JSON handler in `cmd/server`). Do not introduce a second logging library. Request logging is `internal/api/middleware`. Worker logs include a `worker` id.

## Config

All runtime knobs are env vars on `config.Config` with `env` / `envDefault` tags (`caarlos0/env`). Validate invariants in `config.Load` (for example concurrency and max ingest months >= 1). Document new variables in the README table.
