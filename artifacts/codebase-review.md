# Codebase review: reuse, antipatterns, comments

Reviewed `origin/main` at `292e806` (includes the travel-window rebuild queue from #32). Read-only. No application code was changed.

Lenses: duplicated logic that should share helpers, behavior that fights multi-replica workers or this repo's own skills, and comments that do not stand alone. Style nits, wishlist features, and scheduled-ingest design are out of scope.

## Executive summary

The service already has the right skeleton for more than one API and worker replica: job claim uses `FOR UPDATE SKIP LOCKED`, queueing takes `pg_advisory_xact_lock`, travel-window rebuilds serialize on a separate lock from queueing, and HTTP handlers queue work instead of downloading. Rate limits live in Postgres, API-key hashes stay out of JSON, and 500 bodies use stable strings.

The gaps are places that pattern was copied instead of shared, and a few paths that look locked but are not. Flight-performance and weather ingest handlers, and their `Create*IngestJob` store methods, are near-copies. A multi-month POST commits each month in its own transaction, so a later 409 or 500 leaves earlier months queued. Losing a job lease cancels the worker only when a later heartbeat observes `ErrJobStatusConflict`; until then a second replica can run the same job, and month `COPY` does not take the advisory lock that queueing uses. `GET /api/v1/jobs` loads flight and weather details one job at a time. The on-time rule is pasted through six stats queries plus the travel-window insert, with a Go `30` and a SQL `"30"` kept aligned by a `strings.Contains` test the store skill tells us not to write. IEM's one-request-per-second throttle is an in-process mutex, so extra replicas multiply traffic to IEM.

Comments are mostly good. The weak ones point at another query or another file for the actual rule. The new lock-key comment on `TravelWindowRebuildLockKey` is the pattern to copy.

## 1. Code reuse

### P1 — Month ingest queue is copied twice, and the parameterless queue was only half-extracted

**Where:** `IngestHandler.Create` (`internal/api/handlers/ingest.go`), `WeatherIngestHandler.Create` (`internal/api/handlers/weather_ingest.go`), `postgres.Store.CreateFlightPerformanceIngestJob` and `CreateWeatherIngestJob` (`internal/store/postgres/jobs.go`). Related, already shared: `ReferenceIngestHandler.create` (`internal/api/handlers/reference_ingest.go`), `RebuildTravelWindowsHandler.Create` (`internal/api/handlers/travel_window_rebuild.go`), `postgres.Store.insertPendingTypeJob` (`internal/store/postgres/reference.go`).

**What's wrong:** The two month handlers repeat the same sequence: decode JSON, `Validate`, `ingest.ExpandMonths`, `writeIngestRangeError`, active-month check, optional exists-data check, per-month `Create*`, map `store.ErrActiveIngestConflict` to 409, append a job response, return 201. The two store methods repeat begin / `QueryAdvisoryXactLock(jobType, year*100+month)` / scan active months / `execCreateJob` / insert detail / commit. Weather adds stations JSON; that is the only real difference.

`insertPendingTypeJob` already collapsed reference ingest and travel-window rebuild queueing. The HTTP layer did not follow. `RebuildTravelWindowsHandler.Create` repeats the active-check / 409 / insert / 201 shape, with its own response struct (`RebuildTravelWindowsJobResponse`) that matches `ReferenceIngestJobResponse` (`id`, `type`, `status`). `FlightPerformanceIngestConflictResponse` and `WeatherIngestConflictResponse` in `internal/api/handlers/swagger_models.go` are the same struct twice.

**Why it matters:** The next dataset will be a third paste. The partial-commit bug below lives in both month handlers because the loop was copied. #32 shows the store-side fix and leaves the handler-side copy in place.

**Fix:** Add `insertPendingMonthJob` next to `insertPendingTypeJob`, parameterized by job type, year, month, and an optional detail insert. Add one handler helper for "expand range, 409 if active, 409 if data exists unless force, create one job per month inside one transaction" (see the antipattern finding). Keep per-handler swag comments; swag needs those on the exported methods. One conflict struct can back both month 409s. One job-summary struct can back reference and rebuild 201s.

Do not try to DRY the swag comment blocks themselves.

### P1 — On-time classification and the carrier sample floor are copied instead of shared

**Where:** `internal/store/queries.go` — the `is_cancelled` / `is_diverted` / `is_delayed` block in `QueryRouteStats`, `QueryRouteStatsCarrierOnTime`, `QueryCarrierStats`, `QueryCarrierStatsRoutes`, `QueryCarrierStatsAirports`, and `QueryRouteOutlook`. The same predicate is inlined again as `is_on_time` in `QueryInsertRouteTravelWindows`. `carrierStatsMinSampleSQL = "30"` in that file versus `CarrierStatsMinSampleSize = 30` in `internal/store/carrier_agg.go`. The Go mirror is `classifyRouteStatsOnTime` in `internal/store/route_agg_test.go`.

**What's wrong:** `sqlPrimaryCauseCase` is already a shared fragment. The classification CTE that every stats endpoint depends on is not. Route stats, carrier stats, outlook, and travel windows can drift independently. The sample floor is a Go int used by `RankCarrierRoutes` / `RankCarrierAirports` and a separate SQL literal used in `HAVING`. `TestCarrierStatsMinSampleSQLMatchesConstant` (`internal/store/carrier_agg_test.go`) exists to notice that drift.

**Why it matters:** These queries are the product. A one-line change to delay priority in `QueryRouteStats` will not update outlook, carrier best/worst, or travel windows. The skill in `.cursor/skills/store-and-migrations/SKILL.md` and `development-practices/testing.md` says not to unit-test static `Query*` strings with `strings.Contains`. That test is the glue for a constant that should have been single-sourced. Builder tests in `internal/store/postgres/routes_query_test.go` are the allowed kind (they assert the dynamic `WHERE`); this one is not.

**Fix:** One unexported SQL fragment for the three booleans, concatenated the way `sqlPrimaryCauseCase` already is. State the on-time rule once next to that fragment. Build the carrier `HAVING COUNT(*) >= N` in `buildCarrierStatsQuery` from `store.CarrierStatsMinSampleSize`, and delete `TestCarrierStatsMinSampleSQLMatchesConstant`. Keep `classifyRouteStatsOnTime` only if a test still needs a Go oracle; point it at the fragment's rule, not at query names.

### P2 — Smaller copies

| Location | What is repeated | Suggestion |
| --- | --- | --- |
| `postgres.Store.RouteStats` and `scanCarrierOverall` (`internal/store/postgres/routes.go`, `carriers.go`) | Same delay-average / cause-share scan and `ratio(...)` assignments | One scanned struct and `applyDelayStats` |
| `validateRouteStatsSpan` and `validateCarrierStatsSpan` (`internal/api/query/routes.go`, `carriers.go`) | Same inclusive day-count check against `store.MaxStatsSpanDays` | One helper `reportDateSpan(sl, start, end, field)` |
| `MonthsWithFlightPerformanceData` and `MonthsWithWeatherData` (`internal/store/postgres/jobs.go`) | One `SELECT 1 ... LIMIT 1` per requested month (up to `MAX_INGEST_MONTHS`, default 24) | One helper, one query: `WHERE (year, month) IN (...)` |
| `ActiveFlightPerformanceIngestMonths` and `ActiveWeatherIngestMonths` | Same `collectRequestedActiveMonths` wrapper. The flight method returns early on an empty list; the weather method does not | One method that takes the SQL constant. Keep the empty-list short-circuit for both |
| `config.Load` (`internal/config/config.go`) | Six identical `>= 1` checks on RPM fields | A small table of field name to value |
| `bts.ImportResult.MarshalJSON` and `iem.ImportResult.MarshalJSON` | Same `year` / `month` / `rows_imported` object | A shared ingest result type, or struct tags if the custom marshaler is only renaming fields |
| `decodeForceIngestRequest` and `decodeEmptyBody` (`reference_ingest.go`, `travel_window_rebuild.go`) | Two JSON body decoders. Only the rebuild one sets `DisallowUnknownFields` and rejects trailing values | One decoder with a "empty allowed" and "unknown fields rejected" option |

`replaceTable` in `internal/store/postgres/copy.go` is a one-argument forwarder to `replaceTables`. `.cursor/skills/development-practices/style.md` says not to add that kind of wrapper. Call `replaceTables` from the three `Replace*` methods.

## 2. Antipatterns

### P1 — A multi-month ingest POST commits month by month

**Where:** The `for _, ym := range months` loops in `IngestHandler.Create` and `WeatherIngestHandler.Create`. Each `CreateFlightPerformanceIngestJob` / `CreateWeatherIngestJob` commits its own transaction.

**What's wrong:** The active-month check before the loop is not the lock. The lock inside each `Create*` is. If month 3 of 12 hits `ErrActiveIngestConflict` or a database error, months 1 and 2 are already pending. The client gets 409 or 500 and no `jobs` array. A retry then 409s on the months that actually queued.

**Why it matters:** Operators cannot tell a failed request from a partial queue without listing jobs. Two replicas racing on an overlapping range make this routine, not theoretical: the per-month advisory lock does its job and the handler treats that as a failed request.

**Fix:** One store method that, in a single transaction, takes `pg_advisory_xact_lock` for every requested month, rechecks conflicts, inserts every job, and commits once. On conflict, roll back the whole batch and return the conflicting months. Preserve the current 409 bodies (`active_ingest_months` vs `existing_data_months`, force skips only the exists check).

### P1 — A lost lease does not stop the worker, and month replace is unlocked

**Where:** `Worker.runHeartbeat` and `Worker.poll` (`internal/operator/worker.go`), `Processor.Process` (`internal/operator/processor.go`), `ReplaceFlightPerformanceByMonth` / `ReplaceWeatherObservationsByMonth` (`internal/store/postgres/copy.go`). Claim itself is fine: `QueryClaimNextPendingJobSelect` is `FOR UPDATE SKIP LOCKED`.

**What's wrong:** `poll` runs the job on `context.WithCancelCause(context.Background())`, not on the worker context. Shutdown cancels that context explicitly. Heartbeat cancels it only when `HeartbeatJob` returns `store.ErrJobStatusConflict`. Any other heartbeat error is logged and the loop continues. `ResetStaleRunningJobs` (startup `RecoverStaleJobs` and `reclaimLoop`) will set that row back to `pending` once `lease_expires_at` passes (default TTL 90s, heartbeat and reclaim every ~30s). Another replica can claim it. The original worker keeps downloading and `COPY`ing until a later heartbeat both succeeds and notices the status change. If heartbeats keep failing, it never notices.

Month replace deletes and copies in a transaction, and it does not take `QueryAdvisoryXactLock` for `(job type, year*100+month)`. Two replaces of the same month serialize on row locks and the last commit wins. Both still download. Each successful flight-performance job then calls `RebuildRouteTravelWindows`, so the earlier worker can commit rollups from its snapshot before the later replace lands.

**Why it matters:** This is the multi-replica hole next to an otherwise careful lease design. Default `BTS_DOWNLOAD_TIMEOUT` and `IEM_ASOS_DOWNLOAD_TIMEOUT` are 10 minutes, so a job routinely outlives one lease period and depends on heartbeats.

**Fix:** On heartbeat failure, if `time.Now()` is past the lease this worker last wrote, cancel the job context immediately with a distinct cause. Keep the status-conflict abort. Inside `replaceTables`, take the same advisory lock key the create path uses (`job type` plus `year*100+month`) so a reclaimed duplicate waits until the first transaction rolls back or commits. Do not hold that lock across the download; take it only around delete+`COPY`.

### P1 — `GET /api/v1/jobs` is N+1

**Where:** `JobsHandler.List` and `toJobResponse` (`internal/api/handlers/jobs.go`). `ListJobs` is one query (`QueryListJobs`). For `import_flight_performance` and `import_weather_observations`, `toJobResponse` then calls `GetFlightPerformanceIngestJob` or `GetWeatherIngestJob`.

**What's wrong:** `limit` defaults to 50 and allows 500 (`internal/api/query/jobs.go`). A page of flight and weather jobs is one list query plus one detail query per row. Reference and rebuild jobs skip the extra query, so the cost depends on the mix.

**Why it matters:** This is the admin hot path for "what is running". It is chatty under the skill's own rule that reads should not fan out per row when the detail tables are 1:1 with `jobs`.

**Fix:** Extend `QueryListJobs` with `LEFT JOIN flight_performance_ingest_jobs` and `LEFT JOIN weather_ingest_jobs` (or a `ListJobs` result that already carries year, month, and stations). `Get` can keep two queries.

### P1 — IEM's 1 request/second cap is per process

**Where:** `Downloader.waitTurn` (`internal/ingest/iem/downloader.go`). `minInterval` defaults to one second. The clock is `d.mu` and `d.lastRequest` on the process-local downloader. `cmd/server/main.go` builds one downloader per process. `WORKER_CONCURRENCY` is covered by that mutex. Other replicas are not.

**What's wrong:** The ingest skill says IEM downloads are throttled to about one request per second and retry HTTP 503. That throttle does not cross process boundaries. Two API replicas with workers are two requests per second, plus retries (`maxAttempts` 6, backoff `attempt * 5s`).

**Why it matters:** Job leases, advisory locks, and `ConsumeRateLimit` were built so more than one replica can run. This is the external dependency that still assumes one process.

**Fix:** Pace IEM with a shared lock or the existing rate-limit table (a fixed bucket key, capacity 60/minute or a 1-second advisory lock with `pg_advisory_lock` / try-lock and sleep). Keep the in-process mutex as a local burst shield in front of that, the same split `burstShield` already documents for HTTP.

### P2 — Unparseable dates become an empty 200

**Where:** `postgres.Store.RouteOutlook` (`internal/store/postgres/routes.go`): `time.Parse("2006-01-02", analysisEnd.String)` failure returns `out, nil`. Invalid `filter.DepTime` after `HHMMToMinutes` does the same. `postgres.Store.CarrierStats` (`internal/store/postgres/carriers.go`): `CarrierStatsWindow` returning `ok == false` returns the zero `stats` and a nil error when the caller omitted dates.

**What's wrong:** `MAX(flight_date)` is scanned into a string and must be `YYYY-MM-DD`. A driver format change or a corrupt value becomes a successful empty outlook (sample size 0, no analysis window) or a carrier payload with empty dates and zero flights. The query layer already rejects bad `dep_time`, so the HHMM branch is only reachable for direct store callers, and it still hides the failure.

**Fix:** Return a wrapped error from both paths. Handlers already map unknown errors to a stable 500.

### P2 — Job `error` is the raw wrapped chain; HTTP 500s are not

**Where:** `Processor.Process` stores `err.Error()` via `FailJob`. `JobsHandler.toJobResponse` copies `job.Error` into the JSON. Import errors include `fmt.Errorf("download bts zip: ...")`, `fmt.Errorf("load flights: %w")`, `fmt.Errorf("iem service error: %s", msg)` (`internal/ingest/iem/downloader.go`), and temp paths from `os.Open`.

**What's wrong:** `.cursor/skills/development-practices/style.md` says HTTP 500 bodies stay client-safe. Handlers follow that (`errFailedCreateIngestJob`, "failed to compute route stats", and so on). The jobs API, which is admin-only, returns the unfiltered chain: driver text, upstream response snippets, local paths.

**Fix:** Log the full error on the worker. Persist a short stable reason (and, if operators need it, a separate redacted detail). Keep shutdown's `"interrupted by shutdown"` as the one fixed string it already is.

### P2 — `CreateJob` and `UpdateJob` skip the guards every real write uses

**Where:** `Store.CreateJob` and `Store.UpdateJob` (`internal/store/store.go`, `internal/store/postgres/postgres.go`). No handler, worker, or ingest service calls them. `QueryUpdateJob` updates type, status, result, and error by id with no `status = 'running'` predicate and does not touch `lease_expires_at`.

**What's wrong:** Every production insert goes through a locked `Create*` method. Every production status change goes through `ClaimNextPendingJob`, `CompleteJob`, `FailJob`, `HeartbeatJob`, or `ResetStaleRunningJobs`. These two methods are a second write path on the interface the stub must still implement.

**Fix:** Remove them from `Store` and the stub, or make `UpdateJob` impossible to use for a status change (do not export it). `execCreateJob` can stay unexported for the locked inserts.

### P2 — Job type is a string, so switches cannot be exhaustive

**Where:** `model.Job.Type` is `string`. Constants live in `internal/model/job.go` (`JobTypeImportFlightPerformance`, `JobTypeRebuildRouteTravelWindows`, and the rest). `Processor.handlers` is `map[string]JobHandler`. `JobsHandler.toJobResponse` switches on two types and ignores the rest. `NewProcessor` overwrites the map on a duplicate `Type()` with no error.

**What's wrong:** `.cursor/skills/development-practices/style.md` wants exhaustive enum switches. A new month-scoped job type compiles, is claimable, and `List` omits year and month. Two handlers registered with the same string drop the first one silently.

**Fix:** `type JobType string`, use it on `model.Job` and `JobHandler.Type()`, and give `toJobResponse` a default that returns an error for an unknown type. `NewProcessor` should fail on a duplicate type.

### P2 — Rate-limit cleanup swallows errors on the request path

**Where:** `postgres.Store.deleteStaleRateLimitBuckets` (`internal/store/postgres/ratelimit.go`), called from `ConsumeRateLimit`. About 1 in 64 calls (via `UnixNano()&63`) runs `QueryDeleteStaleRateLimitBuckets`. The result is `_ =`.

**What's wrong:** A failing cleanup is invisible, and the delete shares the request context and a connection with authentication. `writeJSON` (`internal/api/handlers/health.go`) likewise drops `json.Encoder.Encode` errors after the status is already written.

**Fix:** Move bucket deletion to the worker reclaim loop and log the error. For `writeJSON`, log the encode error; the status cannot be changed after `WriteHeader`.

### P2 — BTS downloads the whole zip into memory

**Where:** `bts.Downloader.DownloadCSV` (`internal/ingest/bts/downloader.go`) `io.ReadAll`s the body, checks the PK header, then `os.WriteFile`s it.

**What's wrong:** The worker is the right place for this download. The buffer is not. A month zip lives on the heap for every concurrent flight job (`WORKER_CONCURRENCY`, default 2) before it hits disk.

**Fix:** Stream the body to the temp file and sniff the first four bytes. Keep the zip magic check.

## 3. Comments

No `TODO`, "as above", or "same as the other handler" comments turned up in Go. The weak comments are cross-references that omit the rule, or that sound like they cover more code than they do.

### P2 — Travel-window on-time comment points at another query

**Where:** `QueryInsertRouteTravelWindows` in `internal/store/queries.go`:

> On-time matches QueryRouteStats: not cancelled, not diverted, arr_del15 < 1.

**What's wrong:** The sentence is false until the reader opens `QueryRouteStats` and checks precedence (cancelled before diverted before `arr_del15`). The insert uses a different shape (`is_on_time` boolean) than the stats CTEs. If `QueryRouteStats` changes, this comment becomes a lie with nothing to fail the build.

**Rewrite:** "A flight counts as on time when cancelled < 1, diverted < 1, and arr_del15 < 1. Cancelled is applied first, then diverted, then arrival delay. Route stats, carrier stats, outlook, and this insert share that rule."

After the shared SQL fragment in the reuse finding, this comment can shrink to one line that names the fragment.

### P2 — Flight-import comment describes only the rebuild

**Where:** `FlightPerformanceIngestHandler.Process` (`internal/operator/flight_performance_ingest_handler.go`):

> Rebuild typical-year rollups from flight_performance after a successful month replace. RebuildRouteTravelWindows is multi-replica safe (advisory lock) and idempotent (full replace from source).

**What's wrong:** The sentences are accurate about `RebuildRouteTravelWindows` and incomplete about the function they sit in. `ImportMonth` has already committed in another transaction with no advisory lock. "Full replace from source" does not name `route_travel_window_scopes` and `route_travel_window_buckets`. The lock id is `store.TravelWindowRebuildLockKey`, which is not the queue lock.

**Rewrite:** "The month COPY is already committed. Rebuild `route_travel_window_scopes` and `route_travel_window_buckets` from `flight_performance` under `pg_advisory_xact_lock(TravelWindowRebuildLockKey)`. The rebuild truncates and reinserts, so a second caller repeats the same result. Queueing uses `TravelWindowRebuildJobLockKey` and does not hold the rebuild lock."

### P2 — Test helper comment leads with query names

**Where:** `classifyRouteStatsOnTime` in `internal/store/route_agg_test.go`:

> mirrors the SQL in QueryRouteStats and QueryRouteStatsCarrierOnTime: cancelled takes priority, then diverted, then delayed when arr_del15 >= 1.

**What's wrong:** The rule after the colon stands alone. The query list does not: carrier stats, outlook, and travel windows use the same rule and are omitted, so the comment looks narrower than the behavior.

**Rewrite:** "On-time is the operated flights that are not cancelled, not diverted, and have arr_del15 < 1. Cancelled wins, then diverted, then delay."

### Comments that already stand alone

Keep these. They explain a lock, a protocol quirk, or a non-obvious SQL fact without sending the reader elsewhere:

- `TravelWindowRebuildLockKey` / `TravelWindowRebuildJobLockKey` in `internal/store/travel_window.go` (queue lock versus rebuild lock, and why a POST must not wait for the rebuild).
- `insertPendingTypeJob` in `internal/store/postgres/reference.go` (hashtext namespace, second key 0 for type-only jobs).
- `QueryAdvisoryXactLock` in `internal/store/queries.go` (why a check-then-insert under READ COMMITTED is not enough).
- `QueryRouteIdentity` comment (date, flight number, and weekday are filters, not identity).
- `postgres.Store.RouteStats` (a `COUNT(*)` of zero is not `sql.ErrNoRows`).
- `clientIP` in `internal/api/middleware/clientip.go` (trust proxy, right-most `X-Forwarded-For` hop, single proxy).
- `burstShield` in `internal/api/middleware/shield.go` (per-replica shed; advertised caps come from Postgres).
- `FreshnessInput` in `internal/store/freshness.go` (empty job id, month versus snapshot, `airport_weather_stations` as a sibling table).
- `TravelWindowBounds` (the `GREATEST(min_date, max_date - 2 years)` rule is written out, not only named).

## Looks solid

Leave these alone.

- **Claim and lease protocol.** `ClaimNextPendingJob` is `SELECT ... FOR UPDATE SKIP LOCKED` plus a conditional update. `CompleteJob`, `FailJob`, and `HeartbeatJob` all require `status = running` and return `ErrJobStatusConflict` otherwise. Startup and the reclaim loop both call `ResetStaleRunningJobs`. Shutdown writes a fixed `"interrupted by shutdown"` and uses `context.WithoutCancel` so the status write survives cancellation.
- **Queue locks for parameterless jobs.** `insertPendingTypeJob` is the right extraction. Rebuild queueing uses a different lock key from the rebuild itself, and the comment says why.
- **Travel-window rebuild.** Truncate plus insert under `TravelWindowRebuildLockKey` is idempotent. Flight import and the rebuild job can both call it.
- **HTTP versus workers.** Ingest POSTs validate, 409, and insert `pending` jobs. Downloads live in `internal/ingest/<provider>` behind `WithCSVOpener`. Tests use fixtures and `httptest`, not TranStats, IEM, or GitHub.
- **COPY loads.** `replaceTables` deletes then `COPY`s. Reference tables and weather stations go through that helper. No per-row ingest inserts.
- **Stats SQL construction.** `buildRouteStatsQuery` and `buildCarrierStatsQuery` substitute placeholders and append args. User values are not concatenated into SQL. Dynamic-builder tests belong in `internal/store/postgres/routes_query_test.go`.
- **Auth and secrets.** `APIKey.KeyHash` is `json:"-"`. List and create responses use `APIKeyResponse`, which has no hash. Unknown keys still run `VerifyAPIKey` against a dummy hash. Bootstrap logs the prefix only. `AUTH_DISABLED` logs a production warning.
- **Shared rate limit.** `QueryConsumeRateLimit` is one statement for every replica. The in-process shield is documented as a shed, not the advertised cap. Middleware uses `handlers.WriteError` and the exported stable strings.
- **Handler errors.** Store failures on reads and ingests use fixed messages. `errors.Is` maps `ErrNotFound` to 404 and `ErrActiveIngestConflict` to 409. `writeJSON` is the only encoder.
- **Store stub.** Unset `Fn` fields panic. `var _ store.Store = (*Stub)(nil)` is the compile-time check the skill asks for.
- **Reference ingest HTTP.** `createReference` / `create` is the shared queue path for countries, regions, airports, and weather stations, including empty-body `{"force":false}`.
- **Worker logging and wiring.** `cmd/server` is the composition root. Worker logs include a `worker` id. `slog` is the only logger.

## Suggested fix order

Each item is a small PR. Later items should not start until the lease and enqueue behavior is settled, because they touch the same job paths.

1. **One transaction for a multi-month enqueue.** Store method plus the two handlers. Same 409 bodies. Add a handler test where the third month conflicts and the stub shows the first two were not committed.
2. **Abort on a lost lease, and lock month replace.** Heartbeat cancels when the lease is already expired, even if the update failed for another reason. `replaceTables` takes the create-path advisory lock. Unit-test the cancel cause with the worker stub; the lock call can be asserted through a store stub or a SQL-builder seam if the lock stays a fixed `QueryAdvisoryXactLock` execution.
3. **List jobs in one query.** Join the two detail tables. Handler test: a page with one flight job and one weather job calls the detail methods zero times.
4. **One on-time SQL fragment and one sample-size constant.** Delete `TestCarrierStatsMinSampleSQLMatchesConstant`. Adjust the travel-window comment in the same PR so it states the rule.
5. **Shared IEM pacing.** Advisory lock or the rate-limit table. Keep the in-process mutex. Existing `httptest` downloader tests should still pass with `minInterval` 0.
6. **Dead job writes and job-type exhaustiveness.** Remove or unexport `CreateJob` / `UpdateJob`. Introduce `JobType` and a duplicate-registration error. Rewrite the flight-import rebuild comment and the test-helper comment while the job types are in hand.
7. **Small dedupes.** Shared delay-stat scan, date-span validator, month existence query, conflict/response structs, rate-limit cleanup logging. `replaceTable` goes away by calling `replaceTables` directly. Optional: stream the BTS zip to disk.

Swagger annotations stay per handler. `make swagger` only if a response type used in a comment is renamed.
