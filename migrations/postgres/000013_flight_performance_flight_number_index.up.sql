CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_flight_performance_carrier_flight_number_date
    ON flight_performance (iata_code_marketing_airline, flight_number_marketing_airline, flight_date)
    INCLUDE (origin, dest, crs_dep_time);
