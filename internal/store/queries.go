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
		SET status = $1, started_at = $2, updated_at = $3
		WHERE id = $4 AND status = $5`

	QueryCompleteJob = `
		UPDATE jobs
		SET status = $1, result = $2, error = $3, ended_at = $4, updated_at = $5
		WHERE id = $6 AND status = $7`

	QueryFailJob = `
		UPDATE jobs
		SET status = $1, error = $2, ended_at = $3, updated_at = $4
		WHERE id = $5 AND status = $6`

	QueryResetStaleRunningJobs = `
		UPDATE jobs
		SET status = $1, started_at = NULL, updated_at = $2
		WHERE status = $3 AND started_at IS NOT NULL AND started_at < $4`

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

	QueryDeleteAllCountries = `DELETE FROM countries`
	QueryDeleteAllRegions   = `DELETE FROM regions`
	QueryDeleteAllAirports  = `DELETE FROM airports`

	QueryHasCountriesData = `SELECT 1 FROM countries LIMIT 1`
	QueryHasRegionsData   = `SELECT 1 FROM regions LIMIT 1`
	QueryHasAirportsData  = `SELECT 1 FROM airports LIMIT 1`

	QueryMigrationVersion = `
		SELECT version, dirty
		FROM schema_migrations
		LIMIT 1`

	QueryRoutePerfBase = `
		SELECT
			flight_date,
			day_of_week,
			origin,
			dest,
			iata_code_marketing_airline,
			flight_number_marketing_airline,
			crs_dep_time,
			arr_delay_minutes,
			dep_delay_minutes,
			arr_del15,
			dep_del15,
			cancelled,
			cancellation_code,
			diverted,
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
		FROM flight_performance`
)
