---
name: ingest-jobs
description: Add or change flight-tracker ingest adapters, import jobs, background workers, and operator handlers. Use when working on BTS, IEM, OurAirports, csvparse, ingest range expansion, job types, or internal/operator.
---

# Ingest and jobs

Also follow [development-practices](../development-practices/SKILL.md) and [store-and-migrations](../store-and-migrations/SKILL.md) when the store or schema changes.

## Pipeline

```text
POST /api/v1/ingest...  →  validate + 409 checks  →  Create*Job (pending)
Worker.ClaimNextPendingJob  →  operator.JobHandler.Process
    →  ingest/<provider>.Service.Import*
        →  download (or CSVOpener) → csvparse.Parse → store.Replace*
Processor.CompleteJob / FailJob
```

HTTP handlers must not download. Workers must always replace the target month or table; `force` only bypasses the exists-data 409 at queue time.

## Existing providers

| Package | Job type(s) | Replace |
| --- | --- | --- |
| `internal/ingest/bts` | `import_flight_performance` | `ReplaceFlightPerformanceByMonth` |
| `internal/ingest/iem` | `import_weather_observations`, `import_weather_stations` | month observations; full station catalog + mapping |
| `internal/ingest/ourairports` | `import_countries`, `import_regions`, `import_airports` | full table |

Shared helpers:

- `ingest.ExpandMonths` — inclusive month range; omit end = single month; cap `MAX_INGEST_MONTHS`
- `ingest.DownloadFile` — HTTP GET to a temp file + cleanup
- `ingest/csvparse.Parse` — header mapper to canonical DB columns

Earliest supported year is **2018** (`model.MinFlightPerformanceIngestYear` / `MinWeatherIngestYear`).

IEM downloads are throttled (~1 req/s) and retry HTTP 503. Preserve that if you touch the IEM client.

## Adding a job type

Work through this list; skip steps that already exist for an extension of a current provider.

1. **Constant** — `internal/model` `JobTypeImport…` string (`import_snake_case`).
2. **Schema** — job detail table if the job has parameters (year/month/stations). Migration pair under `migrations/postgres`.
3. **Store** — `Create*Job`, active-job check, exists-data check, `Replace*`. SQL in `internal/store/queries.go`. Update `storetest.Stub`.
4. **Adapter** — `internal/ingest/<provider>`:
   - `Downloader` with injectable base URL + timeout (config env vars)
   - Canonical `DBColumns` + header mapper
   - `Service` with `WithCSVOpener` for tests
   - `ImportResult` JSON (`year`/`month`/`rows_imported` or `dataset`/`rows_imported`)
5. **Operator handler** — implement `operator.JobHandler` (`Type()` + `Process`). Load job details from the store, call `Import*`, return `json.Marshal(result)`.
6. **Register** — `operator.NewProcessor(...)` in `cmd/server/main.go`.
7. **HTTP queue endpoint** — same 400/409/201 pattern as `internal/api/handlers/ingest.go` or `reference_ingest.go`. Swag `@Tags ingest,internal`.
8. **Fixture** — small file in `testdata/`. Not a production input, but it should look like production data to the extent that is reasonably possible. Point tests at `WithCSVOpener`, not the network.
9. **README** — ingest behavior + curl example.

## 409 rules (do not weaken)

- Pending or running job for the same month/dataset → 409 (even with `force`).
- Data already present and `force` is false → 409 with the existing conflict body shape.
- `force: true` skips the exists check only.

## Tests

- Adapter: parse fixture, import via stubbed `Replace*`, error when downloader/opener is missing.
- Operator handler: stub store + ingest, assert `Type()` and result JSON.
- HTTP: stub active months / has-data / create-job Fns; assert 201 and both 409 flavors.

Do not add tests that download from TranStats, IEM, or GitHub.
