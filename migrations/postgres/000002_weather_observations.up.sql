CREATE TABLE weather_ingest_jobs (
    job_id TEXT PRIMARY KEY REFERENCES jobs (id) ON DELETE CASCADE,
    year   INTEGER NOT NULL,
    month  INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12)
);

CREATE INDEX idx_weather_ingest_jobs_year_month
    ON weather_ingest_jobs (year, month);

CREATE TABLE weather_observations (
    year    INTEGER NOT NULL,
    month   INTEGER NOT NULL,
    station TEXT NOT NULL,
    valid   TIMESTAMPTZ NOT NULL,
    tmpf    DOUBLE PRECISION,
    dwpf    DOUBLE PRECISION,
    relh    DOUBLE PRECISION,
    drct    DOUBLE PRECISION,
    sknt    DOUBLE PRECISION,
    gust    DOUBLE PRECISION,
    vsby    DOUBLE PRECISION,
    skyc1   TEXT,
    skyc2   TEXT,
    skyc3   TEXT,
    skyl1   DOUBLE PRECISION,
    skyl2   DOUBLE PRECISION,
    skyl3   DOUBLE PRECISION,
    wxcodes TEXT,
    p01i    DOUBLE PRECISION,
    alti    DOUBLE PRECISION,
    mslp    DOUBLE PRECISION,
    metar   TEXT
);

CREATE INDEX idx_weather_observations_station_valid
    ON weather_observations (station, valid);
CREATE INDEX idx_weather_observations_year_month
    ON weather_observations (year, month);
CREATE INDEX idx_weather_observations_valid
    ON weather_observations (valid);
