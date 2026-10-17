-- Sole statement so CREATE INDEX CONCURRENTLY stays outside a transaction.
-- golang-migrate runs each file in one Exec, and PostgreSQL wraps multiple
-- statements in that Exec in a transaction.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_flight_performance_carrier_flight_number_date
    ON flight_performance (iata_code_marketing_airline, flight_number_marketing_airline, flight_date)
    INCLUDE (origin, dest, crs_dep_time);
