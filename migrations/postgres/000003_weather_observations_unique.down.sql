DROP INDEX IF EXISTS idx_weather_observations_station_valid;

CREATE INDEX idx_weather_observations_station_valid
    ON weather_observations (station, valid);
