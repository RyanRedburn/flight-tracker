DROP INDEX IF EXISTS idx_flight_performance_marketing_airline_date;

CREATE INDEX IF NOT EXISTS idx_flight_performance_carrier_stats
    ON flight_performance (iata_code_marketing_airline, flight_date)
    INCLUDE (
        origin, dest, origin_state, dest_state,
        cancelled, diverted, arr_del15,
        arr_delay_minutes, dep_delay_minutes,
        carrier_delay, weather_delay, nas_delay, security_delay, late_aircraft_delay
    );
