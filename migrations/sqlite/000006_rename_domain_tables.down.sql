DROP INDEX IF EXISTS idx_flight_performance_origin_dest_date;
DROP INDEX IF EXISTS idx_flight_performance_year_month;
ALTER TABLE flight_performance RENAME TO on_time_flights;
CREATE INDEX idx_on_time_flights_year_month ON on_time_flights(year, month);
CREATE INDEX idx_on_time_flights_origin_dest_date ON on_time_flights(origin, dest, flight_date);

DROP INDEX IF EXISTS idx_flight_performance_ingest_jobs_year_month;
ALTER TABLE flight_performance_ingest_jobs RENAME TO bts_ingest_jobs;
CREATE INDEX idx_bts_ingest_jobs_year_month ON bts_ingest_jobs(year, month);
