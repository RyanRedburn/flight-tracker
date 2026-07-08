CREATE INDEX idx_on_time_flights_origin_dest_date
  ON on_time_flights (origin, dest, flight_date);
