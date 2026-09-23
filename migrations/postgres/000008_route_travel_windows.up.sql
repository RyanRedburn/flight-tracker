-- Typical-year travel-window rollups at route and route+carrier grain.
-- Rebuilt from flight_performance after each successful flight-performance
-- ingest (advisory-locked full replace; see RebuildRouteTravelWindows).
-- carrier = '' is the route-level (all marketing carriers) grain.

CREATE TABLE route_travel_window_scopes (
    origin       TEXT NOT NULL,
    dest         TEXT NOT NULL,
    carrier      TEXT NOT NULL,
    window_start DATE NOT NULL,
    window_end   DATE NOT NULL,
    flights      INTEGER NOT NULL CHECK (flights >= 1),
    PRIMARY KEY (origin, dest, carrier)
);

CREATE TABLE route_travel_window_buckets (
    origin        TEXT NOT NULL,
    dest          TEXT NOT NULL,
    carrier       TEXT NOT NULL,
    grain         TEXT NOT NULL CHECK (grain IN ('month', 'day_of_week', 'hour')),
    bucket        INTEGER NOT NULL,
    on_time_count INTEGER NOT NULL,
    flights       INTEGER NOT NULL,
    PRIMARY KEY (origin, dest, carrier, grain, bucket),
    CHECK (flights >= 1),
    CHECK (on_time_count >= 0 AND on_time_count <= flights),
    CHECK (
        (grain = 'month' AND bucket BETWEEN 1 AND 12)
        OR (grain = 'day_of_week' AND bucket BETWEEN 1 AND 7)
        OR (grain = 'hour' AND bucket BETWEEN 0 AND 23)
    )
);
