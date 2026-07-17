ALTER TABLE bts_ingest_jobs RENAME TO flight_performance_ingest_jobs;
DROP INDEX IF EXISTS idx_bts_ingest_jobs_year_month;
CREATE INDEX idx_flight_performance_ingest_jobs_year_month ON flight_performance_ingest_jobs(year, month);

ALTER TABLE on_time_flights RENAME TO flight_performance;
DROP INDEX IF EXISTS idx_on_time_flights_year_month;
CREATE INDEX idx_flight_performance_year_month ON flight_performance(year, month);
DROP INDEX IF EXISTS idx_on_time_flights_origin_dest_date;
CREATE INDEX idx_flight_performance_origin_dest_date ON flight_performance(origin, dest, flight_date);
