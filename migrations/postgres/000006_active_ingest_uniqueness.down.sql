DROP TRIGGER IF EXISTS jobs_sync_ingest_job_active ON jobs;
DROP FUNCTION IF EXISTS sync_ingest_job_active();

DROP INDEX IF EXISTS idx_jobs_active_reference_type;
DROP INDEX IF EXISTS idx_weather_ingest_jobs_active_year_month;
DROP INDEX IF EXISTS idx_flight_performance_ingest_jobs_active_year_month;

ALTER TABLE weather_ingest_jobs
    DROP COLUMN IF EXISTS active;

ALTER TABLE flight_performance_ingest_jobs
    DROP COLUMN IF EXISTS active;
