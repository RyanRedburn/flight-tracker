CREATE TABLE weather_stations (
    sid           TEXT PRIMARY KEY,
    network       TEXT NOT NULL,
    name          TEXT,
    tzname        TEXT,
    latitude      DOUBLE PRECISION,
    longitude     DOUBLE PRECISION,
    archive_begin DATE
);

CREATE TABLE airport_weather_stations (
    airport_code TEXT PRIMARY KEY,
    iem_sid      TEXT,
    tzname       TEXT,
    matched      BOOLEAN NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);
