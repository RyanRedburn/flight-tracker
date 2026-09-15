ALTER TABLE flight_performance_ingest_jobs
    ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE weather_ingest_jobs
    ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE flight_performance_ingest_jobs AS b
SET active = FALSE
FROM jobs AS j
WHERE b.job_id = j.id
    AND j.status NOT IN ('pending', 'running');

UPDATE weather_ingest_jobs AS b
SET active = FALSE
FROM jobs AS j
WHERE b.job_id = j.id
    AND j.status NOT IN ('pending', 'running');

-- If a TOCTOU race already created duplicate active months, keep one row.
WITH keep_flight AS (
    SELECT DISTINCT ON (year, month) job_id
    FROM flight_performance_ingest_jobs
    WHERE active
    ORDER BY year, month, job_id
)
UPDATE flight_performance_ingest_jobs AS b
SET active = FALSE
WHERE b.active
    AND NOT EXISTS (
        SELECT 1
        FROM keep_flight AS k
        WHERE k.job_id = b.job_id
    );

WITH keep_weather AS (
    SELECT DISTINCT ON (year, month) job_id
    FROM weather_ingest_jobs
    WHERE active
    ORDER BY year, month, job_id
)
UPDATE weather_ingest_jobs AS b
SET active = FALSE
WHERE b.active
    AND NOT EXISTS (
        SELECT 1
        FROM keep_weather AS k
        WHERE k.job_id = b.job_id
    );

CREATE UNIQUE INDEX idx_flight_performance_ingest_jobs_active_year_month
    ON flight_performance_ingest_jobs (year, month)
    WHERE active;

CREATE UNIQUE INDEX idx_weather_ingest_jobs_active_year_month
    ON weather_ingest_jobs (year, month)
    WHERE active;

CREATE UNIQUE INDEX idx_jobs_active_reference_type
    ON jobs (type)
    WHERE status IN ('pending', 'running')
        AND type IN (
            'import_countries',
            'import_regions',
            'import_airports',
            'import_weather_stations'
        );

CREATE FUNCTION sync_ingest_job_active()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status IN ('pending', 'running') THEN
        UPDATE flight_performance_ingest_jobs SET active = TRUE WHERE job_id = NEW.id;
        UPDATE weather_ingest_jobs SET active = TRUE WHERE job_id = NEW.id;
    ELSE
        UPDATE flight_performance_ingest_jobs SET active = FALSE WHERE job_id = NEW.id;
        UPDATE weather_ingest_jobs SET active = FALSE WHERE job_id = NEW.id;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER jobs_sync_ingest_job_active
AFTER UPDATE OF status ON jobs
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION sync_ingest_job_active();
