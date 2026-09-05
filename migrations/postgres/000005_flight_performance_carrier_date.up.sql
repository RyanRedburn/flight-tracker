CREATE INDEX idx_flight_performance_marketing_airline_date
    ON flight_performance (iata_code_marketing_airline, flight_date);
