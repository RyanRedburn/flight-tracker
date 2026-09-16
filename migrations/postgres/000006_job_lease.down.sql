DROP INDEX IF EXISTS idx_jobs_running_lease;

ALTER TABLE jobs DROP COLUMN IF EXISTS lease_expires_at;
