DELETE FROM weather_observations a
    USING weather_observations b
WHERE a.ctid < b.ctid
    AND a.station = b.station
    AND a.valid = b.valid;

DROP INDEX IF EXISTS idx_weather_observations_station_valid;

CREATE UNIQUE INDEX idx_weather_observations_station_valid
    ON weather_observations (station, valid);
