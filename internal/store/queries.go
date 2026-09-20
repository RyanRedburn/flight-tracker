package store

const (
	QueryCreateJob = `
		INSERT INTO jobs (id, type, status, result, error, created_at, updated_at, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	QueryGetJob = `
		SELECT id, type, status, result, error, created_at, updated_at, started_at, ended_at
		FROM jobs
		WHERE id = $1`

	QueryListJobs = `
		SELECT id, type, status, result, error, created_at, updated_at, started_at, ended_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1`

	QueryUpdateJob = `
		UPDATE jobs
		SET type = $1, status = $2, result = $3, error = $4, updated_at = $5, started_at = $6, ended_at = $7
		WHERE id = $8`

	QueryClaimNextPendingJobSelect = `
		SELECT id, type, status, result, error, created_at, updated_at, started_at, ended_at
		FROM jobs
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`

	QueryClaimNextPendingJobUpdate = `
		UPDATE jobs
		SET status = $1, started_at = $2, updated_at = $3, lease_expires_at = $4
		WHERE id = $5 AND status = $6`

	QueryCompleteJob = `
		UPDATE jobs
		SET status = $1, result = $2, error = $3, ended_at = $4, updated_at = $5, lease_expires_at = NULL
		WHERE id = $6 AND status = $7`

	QueryFailJob = `
		UPDATE jobs
		SET status = $1, error = $2, ended_at = $3, updated_at = $4, lease_expires_at = NULL
		WHERE id = $5 AND status = $6`

	QueryHeartbeatJob = `
		UPDATE jobs
		SET lease_expires_at = $1, updated_at = $2
		WHERE id = $3 AND status = $4`

	QueryResetStaleRunningJobs = `
		UPDATE jobs
		SET status = $1, started_at = NULL, lease_expires_at = NULL, updated_at = $2
		WHERE status = $3 AND lease_expires_at IS NOT NULL AND lease_expires_at < $4`

	// Transaction-scoped lock: hashtext(job_type) + year*100+month (0 for type-only jobs).
	// A plain check-then-insert in a transaction is not enough under READ COMMITTED.
	QueryAdvisoryXactLock = `SELECT pg_advisory_xact_lock(hashtext($1), $2::int)`

	QueryCreateFlightPerformanceIngestJob = `
		INSERT INTO flight_performance_ingest_jobs (job_id, year, month)
		VALUES ($1, $2, $3)`

	QueryGetFlightPerformanceIngestJob = `
		SELECT job_id, year, month
		FROM flight_performance_ingest_jobs
		WHERE job_id = $1`

	QueryActiveFlightPerformanceIngestMonths = `
		SELECT b.year, b.month
		FROM flight_performance_ingest_jobs b
		INNER JOIN jobs j ON j.id = b.job_id
		WHERE j.status IN ($1, $2)`

	QueryActiveIngestJob = `
		SELECT 1
		FROM jobs
		WHERE type = $1 AND status IN ($2, $3)
		LIMIT 1`

	QueryMonthsWithFlightPerformanceData = `
		SELECT 1
		FROM flight_performance
		WHERE year = $1 AND month = $2
		LIMIT 1`

	QueryDeleteFlightPerformanceByMonth = `
		DELETE FROM flight_performance
		WHERE year = $1 AND month = $2`

	QueryCreateWeatherIngestJob = `
		INSERT INTO weather_ingest_jobs (job_id, year, month, stations)
		VALUES ($1, $2, $3, $4)`

	QueryGetWeatherIngestJob = `
		SELECT job_id, year, month, stations
		FROM weather_ingest_jobs
		WHERE job_id = $1`

	QueryActiveWeatherIngestMonths = `
		SELECT b.year, b.month
		FROM weather_ingest_jobs b
		INNER JOIN jobs j ON j.id = b.job_id
		WHERE j.status IN ($1, $2)`

	QueryMonthsWithWeatherData = `
		SELECT 1
		FROM weather_observations
		WHERE year = $1 AND month = $2
		LIMIT 1`

	QueryDeleteWeatherObservationsByMonth = `
		DELETE FROM weather_observations
		WHERE year = $1 AND month = $2`

	QueryDistinctFlightAirportCodes = `
		SELECT DISTINCT code
		FROM (
			SELECT origin AS code
			FROM flight_performance
			WHERE origin IS NOT NULL AND origin <> ''
			UNION
			SELECT dest AS code
			FROM flight_performance
			WHERE dest IS NOT NULL AND dest <> ''
		) AS airports
		ORDER BY code`

	QueryListAirportIdentifiersByIATA = `
		SELECT iata_code, local_code, icao_code, ident
		FROM airports
		WHERE iata_code = ANY($1)`

	QueryDeleteAllCountries = `DELETE FROM countries`
	QueryDeleteAllRegions   = `DELETE FROM regions`
	QueryDeleteAllAirports  = `DELETE FROM airports`

	QueryHasCountriesData = `SELECT 1 FROM countries LIMIT 1`
	QueryHasRegionsData   = `SELECT 1 FROM regions LIMIT 1`
	QueryHasAirportsData  = `SELECT 1 FROM airports LIMIT 1`

	QueryDeleteAllWeatherStations        = `DELETE FROM weather_stations`
	QueryDeleteAllAirportWeatherStations = `DELETE FROM airport_weather_stations`

	QueryHasWeatherStationsData = `
		SELECT 1
		FROM (
			SELECT 1 FROM weather_stations
			UNION ALL
			SELECT 1 FROM airport_weather_stations
		) AS mapping
		LIMIT 1`

	QueryListAirportWeatherStations = `
		SELECT airport_code, iem_sid, tzname, matched, updated_at
		FROM airport_weather_stations
		ORDER BY airport_code`

	QueryMigrationVersion = `
		SELECT version, dirty
		FROM schema_migrations
		LIMIT 1`

	sqlCauseMax = `GREATEST(COALESCE(carrier_delay, 0), COALESCE(weather_delay, 0), COALESCE(nas_delay, 0), COALESCE(security_delay, 0), COALESCE(late_aircraft_delay, 0))`

	sqlPrimaryCauseCase = `CASE
					WHEN NOT is_delayed THEN NULL
					WHEN ` + sqlCauseMax + ` = 0 THEN 'unattributed'
					WHEN COALESCE(late_aircraft_delay, 0) = ` + sqlCauseMax + ` THEN 'late_aircraft'
					WHEN COALESCE(carrier_delay, 0) = ` + sqlCauseMax + ` THEN 'carrier'
					WHEN COALESCE(nas_delay, 0) = ` + sqlCauseMax + ` THEN 'nas'
					WHEN COALESCE(weather_delay, 0) = ` + sqlCauseMax + ` THEN 'weather'
					ELSE 'security'
				END`

	carrierStatsMinSampleSQL = `30`

	QueryRouteStats = `
		WITH matched AS (
			SELECT
				cancelled,
				diverted,
				arr_del15,
				arr_delay_minutes,
				dep_delay_minutes,
				carrier_delay,
				weather_delay,
				nas_delay,
				security_delay,
				late_aircraft_delay,
				div1_airport,
				div2_airport,
				div3_airport,
				div4_airport,
				div5_airport
			FROM flight_performance
			WHERE origin = $1
				AND dest = $2
				AND flight_date >= $3
				AND flight_date <= $4
				/*extra*/
		),
		classified AS (
			SELECT
				*,
				(COALESCE(cancelled, 0) >= 1) AS is_cancelled,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) >= 1) AS is_diverted,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) < 1 AND COALESCE(arr_del15, 0) >= 1) AS is_delayed
			FROM matched
		),
		with_cause AS (
			SELECT
				*,
				` + sqlPrimaryCauseCase + ` AS primary_cause
			FROM classified
		)
		SELECT
			COUNT(*)::int AS flights,
			COUNT(*) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND NOT is_delayed)::int AS on_time,
			COUNT(*) FILTER (WHERE is_delayed)::int AS delayed,
			COUNT(*) FILTER (WHERE is_cancelled)::int AS cancelled,
			COUNT(*) FILTER (WHERE is_diverted)::int AS diverted,
			AVG(arr_delay_minutes) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND arr_delay_minutes IS NOT NULL) AS avg_arr,
			(
				SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY arr_delay_minutes)
				FROM classified
				WHERE NOT is_cancelled AND NOT is_diverted AND arr_delay_minutes IS NOT NULL
			) AS median_arr,
			AVG(arr_delay_minutes) FILTER (WHERE is_delayed AND arr_delay_minutes IS NOT NULL) AS avg_arr_delayed,
			(
				SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY arr_delay_minutes)
				FROM classified
				WHERE is_delayed AND arr_delay_minutes IS NOT NULL
			) AS median_arr_delayed,
			AVG(dep_delay_minutes) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND dep_delay_minutes IS NOT NULL) AS avg_dep,
			AVG(dep_delay_minutes) FILTER (WHERE is_delayed AND dep_delay_minutes IS NOT NULL) AS avg_dep_delayed,
			AVG(COALESCE(carrier_delay, 0)) FILTER (WHERE is_delayed) AS cause_carrier,
			AVG(COALESCE(weather_delay, 0)) FILTER (WHERE is_delayed) AS cause_weather,
			AVG(COALESCE(nas_delay, 0)) FILTER (WHERE is_delayed) AS cause_nas,
			AVG(COALESCE(security_delay, 0)) FILTER (WHERE is_delayed) AS cause_security,
			AVG(COALESCE(late_aircraft_delay, 0)) FILTER (WHERE is_delayed) AS cause_late,
			COUNT(*) FILTER (WHERE primary_cause = 'carrier')::int AS share_carrier,
			COUNT(*) FILTER (WHERE primary_cause = 'weather')::int AS share_weather,
			COUNT(*) FILTER (WHERE primary_cause = 'nas')::int AS share_nas,
			COUNT(*) FILTER (WHERE primary_cause = 'security')::int AS share_security,
			COUNT(*) FILTER (WHERE primary_cause = 'late_aircraft')::int AS share_late,
			COUNT(*) FILTER (WHERE primary_cause = 'unattributed')::int AS share_unattributed,
			(
				SELECT COALESCE(
					json_agg(json_build_object('airport', airport, 'count', cnt) ORDER BY cnt DESC, airport),
					'[]'::json
				)
				FROM (
					SELECT btrim(airport) AS airport, COUNT(*)::int AS cnt
					FROM classified,
						LATERAL unnest(ARRAY[div1_airport, div2_airport, div3_airport, div4_airport, div5_airport]) AS airport
					WHERE is_diverted AND airport IS NOT NULL AND btrim(airport) <> ''
					GROUP BY btrim(airport)
				) d
			) AS diversion_airports
		FROM with_cause`

	QueryRouteStatsCarrierOnTime = `
		WITH matched AS (
			SELECT
				iata_code_marketing_airline,
				cancelled,
				diverted,
				arr_del15
			FROM flight_performance
			WHERE origin = $1
				AND dest = $2
				AND flight_date >= $3
				AND flight_date <= $4
				/*extra*/
		),
		classified AS (
			SELECT
				iata_code_marketing_airline,
				(COALESCE(cancelled, 0) >= 1) AS is_cancelled,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) >= 1) AS is_diverted,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) < 1 AND COALESCE(arr_del15, 0) >= 1) AS is_delayed
			FROM matched
		)
		SELECT
			COALESCE(iata_code_marketing_airline, '') AS carrier,
			COUNT(*) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND NOT is_delayed)::int AS on_time,
			COUNT(*)::int AS flights
		FROM classified
		GROUP BY 1`

	QueryCarrierStatsMaxDate = `
		SELECT MAX(flight_date)::text
		FROM flight_performance
		WHERE iata_code_marketing_airline = $1`

	QueryCarrierStats = `
		WITH matched AS (
			SELECT
				cancelled,
				diverted,
				arr_del15,
				arr_delay_minutes,
				dep_delay_minutes,
				carrier_delay,
				weather_delay,
				nas_delay,
				security_delay,
				late_aircraft_delay
			FROM flight_performance
			WHERE iata_code_marketing_airline = $1
				AND flight_date >= $2
				AND flight_date <= $3
				/*extra*/
		),
		classified AS (
			SELECT
				*,
				(COALESCE(cancelled, 0) >= 1) AS is_cancelled,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) >= 1) AS is_diverted,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) < 1 AND COALESCE(arr_del15, 0) >= 1) AS is_delayed
			FROM matched
		),
		with_cause AS (
			SELECT
				*,
				` + sqlPrimaryCauseCase + ` AS primary_cause
			FROM classified
		)
		SELECT
			COUNT(*)::int AS flights,
			COUNT(*) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND NOT is_delayed)::int AS on_time,
			COUNT(*) FILTER (WHERE is_delayed)::int AS delayed,
			COUNT(*) FILTER (WHERE is_cancelled)::int AS cancelled,
			COUNT(*) FILTER (WHERE is_diverted)::int AS diverted,
			AVG(arr_delay_minutes) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND arr_delay_minutes IS NOT NULL) AS avg_arr,
			(
				SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY arr_delay_minutes)
				FROM classified
				WHERE NOT is_cancelled AND NOT is_diverted AND arr_delay_minutes IS NOT NULL
			) AS median_arr,
			AVG(arr_delay_minutes) FILTER (WHERE is_delayed AND arr_delay_minutes IS NOT NULL) AS avg_arr_delayed,
			(
				SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY arr_delay_minutes)
				FROM classified
				WHERE is_delayed AND arr_delay_minutes IS NOT NULL
			) AS median_arr_delayed,
			AVG(dep_delay_minutes) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND dep_delay_minutes IS NOT NULL) AS avg_dep,
			AVG(dep_delay_minutes) FILTER (WHERE is_delayed AND dep_delay_minutes IS NOT NULL) AS avg_dep_delayed,
			AVG(COALESCE(carrier_delay, 0)) FILTER (WHERE is_delayed) AS cause_carrier,
			AVG(COALESCE(weather_delay, 0)) FILTER (WHERE is_delayed) AS cause_weather,
			AVG(COALESCE(nas_delay, 0)) FILTER (WHERE is_delayed) AS cause_nas,
			AVG(COALESCE(security_delay, 0)) FILTER (WHERE is_delayed) AS cause_security,
			AVG(COALESCE(late_aircraft_delay, 0)) FILTER (WHERE is_delayed) AS cause_late,
			COUNT(*) FILTER (WHERE primary_cause = 'carrier')::int AS share_carrier,
			COUNT(*) FILTER (WHERE primary_cause = 'weather')::int AS share_weather,
			COUNT(*) FILTER (WHERE primary_cause = 'nas')::int AS share_nas,
			COUNT(*) FILTER (WHERE primary_cause = 'security')::int AS share_security,
			COUNT(*) FILTER (WHERE primary_cause = 'late_aircraft')::int AS share_late,
			COUNT(*) FILTER (WHERE primary_cause = 'unattributed')::int AS share_unattributed
		FROM with_cause`

	QueryCarrierStatsRoutes = `
		WITH matched AS (
			SELECT
				origin,
				dest,
				cancelled,
				diverted,
				arr_del15
			FROM flight_performance
			WHERE iata_code_marketing_airline = $1
				AND flight_date >= $2
				AND flight_date <= $3
				/*extra*/
		),
		classified AS (
			SELECT
				origin,
				dest,
				(COALESCE(cancelled, 0) >= 1) AS is_cancelled,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) >= 1) AS is_diverted,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) < 1 AND COALESCE(arr_del15, 0) >= 1) AS is_delayed
			FROM matched
		)
		SELECT
			origin,
			dest,
			COUNT(*)::int AS flights,
			COUNT(*) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND NOT is_delayed)::int AS on_time,
			COUNT(*) FILTER (WHERE is_delayed)::int AS delayed,
			COUNT(*) FILTER (WHERE is_cancelled)::int AS cancelled,
			COUNT(*) FILTER (WHERE is_diverted)::int AS diverted
		FROM classified
		GROUP BY origin, dest
		HAVING COUNT(*) >= ` + carrierStatsMinSampleSQL

	QueryCarrierStatsAirports = `
		WITH matched AS (
			SELECT
				origin,
				dest,
				origin_state,
				dest_state,
				cancelled,
				diverted,
				arr_del15
			FROM flight_performance
			WHERE iata_code_marketing_airline = $1
				AND flight_date >= $2
				AND flight_date <= $3
				/*extra*/
		),
		classified AS (
			SELECT
				origin,
				dest,
				origin_state,
				dest_state,
				(COALESCE(cancelled, 0) >= 1) AS is_cancelled,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) >= 1) AS is_diverted,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) < 1 AND COALESCE(arr_del15, 0) >= 1) AS is_delayed
			FROM matched
		),
		airport_flights AS (
			SELECT origin AS airport, is_cancelled, is_diverted, is_delayed
			FROM classified
			WHERE /*origin_airport*/
			UNION ALL
			SELECT dest AS airport, is_cancelled, is_diverted, is_delayed
			FROM classified
			WHERE /*dest_airport*/
		)
		SELECT
			airport,
			COUNT(*)::int AS flights,
			COUNT(*) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND NOT is_delayed)::int AS on_time,
			COUNT(*) FILTER (WHERE is_delayed)::int AS delayed,
			COUNT(*) FILTER (WHERE is_cancelled)::int AS cancelled,
			COUNT(*) FILTER (WHERE is_diverted)::int AS diverted
		FROM airport_flights
		GROUP BY airport
		HAVING COUNT(*) >= ` + carrierStatsMinSampleSQL

	QueryRouteOutlookMaxDate = `
		SELECT MAX(flight_date)::text
		FROM flight_performance
		WHERE origin = $1
			AND dest = $2
			AND iata_code_marketing_airline = $3`

	QueryRouteOutlook = `
		WITH matched AS (
			SELECT
				cancelled,
				diverted,
				arr_del15,
				arr_delay_minutes,
				dep_delay_minutes
			FROM flight_performance
			WHERE origin = $1
				AND dest = $2
				AND iata_code_marketing_airline = $3
				AND flight_date >= $4::date
				AND flight_date <= $5::date
				AND day_of_week = $6
				AND crs_dep_time IS NOT NULL
				AND crs_dep_time >= 0
				AND crs_dep_time <= 2359
				AND (crs_dep_time / 100) BETWEEN 0 AND 23
				AND (crs_dep_time % 100) BETWEEN 0 AND 59
				AND LEAST(
					ABS(((crs_dep_time / 100) * 60 + (crs_dep_time % 100)) - $7),
					1440 - ABS(((crs_dep_time / 100) * 60 + (crs_dep_time % 100)) - $7)
				) <= $8
		),
		classified AS (
			SELECT
				*,
				(COALESCE(cancelled, 0) >= 1) AS is_cancelled,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) >= 1) AS is_diverted,
				(COALESCE(cancelled, 0) < 1 AND COALESCE(diverted, 0) < 1 AND COALESCE(arr_del15, 0) >= 1) AS is_delayed
			FROM matched
		)
		SELECT
			COUNT(*)::int AS sample_size,
			COUNT(*) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND NOT is_delayed)::int AS on_time,
			COUNT(*) FILTER (WHERE is_delayed)::int AS delayed,
			COUNT(*) FILTER (WHERE is_cancelled)::int AS cancelled,
			COUNT(*) FILTER (WHERE is_diverted)::int AS diverted,
			AVG(arr_delay_minutes) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND arr_delay_minutes IS NOT NULL) AS avg_arr,
			(
				SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY arr_delay_minutes)
				FROM classified
				WHERE NOT is_cancelled AND NOT is_diverted AND arr_delay_minutes IS NOT NULL
			) AS median_arr,
			AVG(arr_delay_minutes) FILTER (WHERE is_delayed AND arr_delay_minutes IS NOT NULL) AS avg_arr_delayed,
			(
				SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY arr_delay_minutes)
				FROM classified
				WHERE is_delayed AND arr_delay_minutes IS NOT NULL
			) AS median_arr_delayed,
			AVG(dep_delay_minutes) FILTER (WHERE NOT is_cancelled AND NOT is_diverted AND dep_delay_minutes IS NOT NULL) AS avg_dep
		FROM classified`
)

const (
	//nolint:gosec // G101: SQL column name key_hash, not a credential
	QueryCreateAPIKey = `
		INSERT INTO api_keys (id, prefix, key_hash, role, name, created_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	//nolint:gosec // G101: SQL column name key_hash, not a credential
	QueryLookupAPIKeyByPrefix = `
		SELECT id, prefix, key_hash, role, name, created_at, revoked_at
		FROM api_keys
		WHERE prefix = $1`

	//nolint:gosec // G101: SQL column name key_hash, not a credential
	QueryGetAPIKey = `
		SELECT id, prefix, key_hash, role, name, created_at, revoked_at
		FROM api_keys
		WHERE id = $1`

	//nolint:gosec // G101: SQL column name key_hash, not a credential
	QueryListAPIKeys = `
		SELECT id, prefix, key_hash, role, name, created_at, revoked_at
		FROM api_keys
		ORDER BY created_at DESC`

	//nolint:gosec // G101: SQL table name api_keys, not a credential
	QueryRevokeAPIKey = `
		UPDATE api_keys
		SET revoked_at = $1
		WHERE id = $2 AND revoked_at IS NULL`

	//nolint:gosec // G101: SQL table name api_keys, not a credential
	QueryCountAPIKeys = `
		SELECT COUNT(*)
		FROM api_keys`

	// Atomic token-bucket consume shared by all API replicas.
	// $1 = bucket_key, $2 = requests per minute (capacity and refill).
	// Tokens are milli-tokens (1000 = 1 request). INSERT admits the first
	// request; ON CONFLICT UPDATE admits when refilled tokens >= 1000.
	QueryConsumeRateLimit = `
		WITH attempt AS (
			INSERT INTO rate_limit_buckets AS b (bucket_key, tokens, last_refill_at)
			VALUES ($1, ($2::int * 1000) - 1000, CLOCK_TIMESTAMP())
			ON CONFLICT (bucket_key) DO UPDATE
			SET
				tokens = LEAST(
					($2::int * 1000)::bigint,
					b.tokens + ((EXTRACT(EPOCH FROM (CLOCK_TIMESTAMP() - b.last_refill_at)) * $2::int * 1000) / 60.0)::bigint
				) - 1000,
				last_refill_at = CLOCK_TIMESTAMP()
			WHERE LEAST(
				($2::int * 1000)::bigint,
				b.tokens + ((EXTRACT(EPOCH FROM (CLOCK_TIMESTAMP() - b.last_refill_at)) * $2::int * 1000) / 60.0)::bigint
			) >= 1000
			RETURNING b.tokens, true AS allowed, CLOCK_TIMESTAMP() AS now_ts
		),
		denied AS (
			SELECT
				LEAST(
					($2::int * 1000)::bigint,
					b.tokens + ((EXTRACT(EPOCH FROM (CLOCK_TIMESTAMP() - b.last_refill_at)) * $2::int * 1000) / 60.0)::bigint
				) AS tokens,
				false AS allowed,
				CLOCK_TIMESTAMP() AS now_ts
			FROM rate_limit_buckets b
			WHERE b.bucket_key = $1
			  AND NOT EXISTS (SELECT 1 FROM attempt)
		)
		SELECT allowed, tokens, now_ts FROM attempt
		UNION ALL
		SELECT allowed, tokens, now_ts FROM denied`

	QueryDeleteStaleRateLimitBuckets = `
		DELETE FROM rate_limit_buckets
		WHERE last_refill_at < $1`
)
