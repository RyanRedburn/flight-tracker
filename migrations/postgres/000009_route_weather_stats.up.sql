-- METAR category columns on weather_observations, plus per-side route weather rollups.
-- ceiling_ft and category are filled by weather ingest in Go
-- (internal/ingest/iem/category.go).
--
-- route_weather_category_buckets is the read model for GET /api/v1/routes/weather-stats.
-- Grain is per side (origin or dest), not an origin-category × dest-category matrix.
-- carrier '' is a missing marketing carrier. flight_number -1 is a missing flight number.
-- UNMATCHED is a rollup bucket only (no classified observation in the ±30 minute window).

ALTER TABLE weather_observations
    ADD COLUMN ceiling_ft INTEGER,
    ADD COLUMN category TEXT;

ALTER TABLE weather_observations
    ADD CONSTRAINT weather_observations_category_chk
    CHECK (category IN (
        'THUNDER', 'LIFR', 'IFR', 'MVFR', 'WINDY', 'PRECIP', 'VFR_FAIR', 'UNKNOWN'
    ));

CREATE TABLE route_weather_category_buckets (
    origin         TEXT NOT NULL,
    dest           TEXT NOT NULL,
    carrier        TEXT NOT NULL,
    flight_date    DATE NOT NULL,
    day_of_week    INTEGER NOT NULL CHECK (day_of_week BETWEEN 1 AND 7),
    flight_number  INTEGER NOT NULL,
    side           TEXT NOT NULL CHECK (side IN ('origin', 'dest')),
    category       TEXT NOT NULL CHECK (category IN (
        'THUNDER', 'LIFR', 'IFR', 'MVFR', 'WINDY', 'PRECIP', 'VFR_FAIR', 'UNKNOWN', 'UNMATCHED'
    )),
    on_time_count  INTEGER NOT NULL,
    flights        INTEGER NOT NULL,
    PRIMARY KEY (origin, dest, carrier, flight_date, flight_number, side, category),
    CHECK (flights >= 1),
    CHECK (on_time_count >= 0 AND on_time_count <= flights)
);

CREATE INDEX idx_route_weather_category_buckets_od_date
    ON route_weather_category_buckets (origin, dest, flight_date);
