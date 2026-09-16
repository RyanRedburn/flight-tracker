ALTER TABLE jobs ADD COLUMN lease_expires_at TIMESTAMPTZ;

-- Give in-flight jobs from pre-lease binaries a short grace matching the default TTL.
UPDATE jobs
SET lease_expires_at = NOW() + INTERVAL '90 seconds'
WHERE status = 'running';

CREATE INDEX idx_jobs_running_lease
    ON jobs (lease_expires_at)
    WHERE status = 'running';
