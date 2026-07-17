package store

const (
	QueryCreateJob = `
		INSERT INTO jobs (id, type, status, result, error, created_at, updated_at, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	QueryGetJob = `
		SELECT id, type, status, result, error, created_at, updated_at, started_at, ended_at
		FROM jobs
		WHERE id = ?`

	QueryListJobs = `
		SELECT id, type, status, result, error, created_at, updated_at, started_at, ended_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT ?`

	QueryUpdateJob = `
		UPDATE jobs
		SET type = ?, status = ?, result = ?, error = ?, updated_at = ?, started_at = ?, ended_at = ?
		WHERE id = ?`

	QueryClaimNextPendingJobSelect = `
		SELECT id, type, status, result, error, created_at, updated_at, started_at, ended_at
		FROM jobs
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1`

	QueryClaimNextPendingJobUpdate = `
		UPDATE jobs
		SET status = ?, started_at = ?, updated_at = ?
		WHERE id = ? AND status = ?`

	QueryCompleteJob = `
		UPDATE jobs
		SET status = ?, result = ?, error = ?, ended_at = ?, updated_at = ?
		WHERE id = ? AND status = ?`

	QueryFailJob = `
		UPDATE jobs
		SET status = ?, error = ?, ended_at = ?, updated_at = ?
		WHERE id = ? AND status = ?`

	QueryResetStaleRunningJobs = `
		UPDATE jobs
		SET status = ?, started_at = NULL, updated_at = ?
		WHERE status = ? AND started_at IS NOT NULL AND started_at < ?`

	QueryCreateFlightPerformanceIngestJob = `
		INSERT INTO flight_performance_ingest_jobs (job_id, year, month)
		VALUES (?, ?, ?)`

	QueryGetFlightPerformanceIngestJob = `
		SELECT job_id, year, month
		FROM flight_performance_ingest_jobs
		WHERE job_id = ?`

	QueryActiveFlightPerformanceIngestMonths = `
		SELECT b.year, b.month
		FROM flight_performance_ingest_jobs b
		INNER JOIN jobs j ON j.id = b.job_id
		WHERE j.status IN (?, ?)`

	QueryActiveIngestJob = `
		SELECT 1
		FROM jobs
		WHERE type = ? AND status IN (?, ?)
		LIMIT 1`

	QueryMonthsWithFlightPerformanceData = `
		SELECT 1
		FROM flight_performance
		WHERE year = ? AND month = ?
		LIMIT 1`

	QueryDeleteFlightPerformanceByMonth = `
		DELETE FROM flight_performance
		WHERE year = ? AND month = ?`

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
