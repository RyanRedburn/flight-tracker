CREATE INDEX CONCURRENTLY idx_flight_performance_origin_dest_carrier_dow_date
    ON flight_performance (origin, dest, iata_code_marketing_airline, day_of_week, flight_date);
