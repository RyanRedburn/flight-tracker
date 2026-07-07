ALTER TABLE jobs ADD COLUMN started_at TEXT;
ALTER TABLE jobs ADD COLUMN ended_at TEXT;
ALTER TABLE jobs DROP COLUMN payload;

CREATE TABLE bts_ingest_jobs (
    job_id TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    year   INTEGER NOT NULL,
    month  INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12)
);

CREATE INDEX idx_bts_ingest_jobs_year_month ON bts_ingest_jobs(year, month);

CREATE INDEX idx_on_time_flights_year_month ON on_time_flights(year, month);
