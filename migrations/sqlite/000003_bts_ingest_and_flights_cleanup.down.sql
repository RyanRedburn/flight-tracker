DROP INDEX IF EXISTS idx_on_time_flights_year_month;
DROP INDEX IF EXISTS idx_bts_ingest_jobs_year_month;
DROP TABLE IF EXISTS bts_ingest_jobs;

ALTER TABLE jobs ADD COLUMN payload TEXT NOT NULL DEFAULT '{}';
ALTER TABLE jobs DROP COLUMN started_at;
ALTER TABLE jobs DROP COLUMN ended_at;
