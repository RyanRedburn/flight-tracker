CREATE TABLE jobs (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    status     TEXT NOT NULL,
    result     JSONB,
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    ended_at   TIMESTAMPTZ
);

CREATE INDEX idx_jobs_status ON jobs (status);
CREATE INDEX idx_jobs_created_at ON jobs (created_at DESC);

CREATE TABLE flight_performance_ingest_jobs (
    job_id TEXT PRIMARY KEY REFERENCES jobs (id) ON DELETE CASCADE,
    year   INTEGER NOT NULL,
    month  INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12)
);

CREATE INDEX idx_flight_performance_ingest_jobs_year_month
    ON flight_performance_ingest_jobs (year, month);

CREATE TABLE flight_performance (
    year INTEGER,
    quarter INTEGER,
    month INTEGER,
    day_of_month INTEGER,
    day_of_week INTEGER,
    flight_date DATE,
    marketing_airline_network TEXT,
    operated_or_branded_code_share_partners TEXT,
    dot_id_marketing_airline INTEGER,
    iata_code_marketing_airline TEXT,
    flight_number_marketing_airline INTEGER,
    originally_scheduled_code_share_airline TEXT,
    dot_id_originally_scheduled_code_share_airline INTEGER,
    iata_code_originally_scheduled_code_share_airline TEXT,
    flight_num_originally_scheduled_code_share_airline INTEGER,
    operating_airline TEXT,
    dot_id_operating_airline INTEGER,
    iata_code_operating_airline TEXT,
    tail_number TEXT,
    flight_number_operating_airline INTEGER,
    origin_airport_id INTEGER,
    origin_airport_seq_id INTEGER,
    origin_city_market_id INTEGER,
    origin TEXT,
    origin_city_name TEXT,
    origin_state TEXT,
    origin_state_fips TEXT,
    origin_state_name TEXT,
    origin_wac INTEGER,
    dest_airport_id INTEGER,
    dest_airport_seq_id INTEGER,
    dest_city_market_id INTEGER,
    dest TEXT,
    dest_city_name TEXT,
    dest_state TEXT,
    dest_state_fips TEXT,
    dest_state_name TEXT,
    dest_wac INTEGER,
    crs_dep_time INTEGER,
    dep_time INTEGER,
    dep_delay DOUBLE PRECISION,
    dep_delay_minutes DOUBLE PRECISION,
    dep_del15 DOUBLE PRECISION,
    departure_delay_groups INTEGER,
    dep_time_blk TEXT,
    taxi_out DOUBLE PRECISION,
    wheels_off INTEGER,
    wheels_on INTEGER,
    taxi_in DOUBLE PRECISION,
    crs_arr_time INTEGER,
    arr_time INTEGER,
    arr_delay DOUBLE PRECISION,
    arr_delay_minutes DOUBLE PRECISION,
    arr_del15 DOUBLE PRECISION,
    arrival_delay_groups INTEGER,
    arr_time_blk TEXT,
    cancelled DOUBLE PRECISION,
    cancellation_code TEXT,
    diverted DOUBLE PRECISION,
    crs_elapsed_time DOUBLE PRECISION,
    actual_elapsed_time DOUBLE PRECISION,
    air_time DOUBLE PRECISION,
    flights DOUBLE PRECISION,
    distance DOUBLE PRECISION,
    distance_group INTEGER,
    carrier_delay DOUBLE PRECISION,
    weather_delay DOUBLE PRECISION,
    nas_delay DOUBLE PRECISION,
    security_delay DOUBLE PRECISION,
    late_aircraft_delay DOUBLE PRECISION,
    first_dep_time INTEGER,
    total_add_g_time DOUBLE PRECISION,
    longest_add_g_time DOUBLE PRECISION,
    div_airport_landings INTEGER,
    div_reached_dest DOUBLE PRECISION,
    div_actual_elapsed_time DOUBLE PRECISION,
    div_arr_delay DOUBLE PRECISION,
    div_distance DOUBLE PRECISION,
    div1_airport TEXT,
    div1_airport_id INTEGER,
    div1_airport_seq_id INTEGER,
    div1_wheels_on INTEGER,
    div1_total_g_time DOUBLE PRECISION,
    div1_longest_g_time DOUBLE PRECISION,
    div1_wheels_off INTEGER,
    div1_tail_num TEXT,
    div2_airport TEXT,
    div2_airport_id INTEGER,
    div2_airport_seq_id INTEGER,
    div2_wheels_on INTEGER,
    div2_total_g_time DOUBLE PRECISION,
    div2_longest_g_time DOUBLE PRECISION,
    div2_wheels_off INTEGER,
    div2_tail_num TEXT,
    div3_airport TEXT,
    div3_airport_id INTEGER,
    div3_airport_seq_id INTEGER,
    div3_wheels_on INTEGER,
    div3_total_g_time DOUBLE PRECISION,
    div3_longest_g_time DOUBLE PRECISION,
    div3_wheels_off INTEGER,
    div3_tail_num TEXT,
    div4_airport TEXT,
    div4_airport_id INTEGER,
    div4_airport_seq_id INTEGER,
    div4_wheels_on INTEGER,
    div4_total_g_time DOUBLE PRECISION,
    div4_longest_g_time DOUBLE PRECISION,
    div4_wheels_off INTEGER,
    div4_tail_num TEXT,
    div5_airport TEXT,
    div5_airport_id INTEGER,
    div5_airport_seq_id INTEGER,
    div5_wheels_on INTEGER,
    div5_total_g_time DOUBLE PRECISION,
    div5_longest_g_time DOUBLE PRECISION,
    div5_wheels_off INTEGER,
    div5_tail_num TEXT,
    duplicate TEXT
);

CREATE INDEX idx_flight_performance_flight_date ON flight_performance (flight_date);
CREATE INDEX idx_flight_performance_origin_dest ON flight_performance (origin, dest);
CREATE INDEX idx_flight_performance_marketing_airline ON flight_performance (iata_code_marketing_airline);
CREATE INDEX idx_flight_performance_year_month ON flight_performance (year, month);
CREATE INDEX idx_flight_performance_origin_dest_date ON flight_performance (origin, dest, flight_date);

CREATE TABLE countries (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    continent TEXT NOT NULL,
    wikipedia_link TEXT,
    keywords TEXT
);

CREATE TABLE regions (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    local_code TEXT,
    name TEXT NOT NULL,
    continent TEXT NOT NULL,
    iso_country TEXT NOT NULL,
    wikipedia_link TEXT,
    keywords TEXT
);

CREATE INDEX idx_regions_iso_country ON regions (iso_country);

CREATE TABLE airports (
    id INTEGER PRIMARY KEY,
    ident TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    latitude_deg DOUBLE PRECISION NOT NULL,
    longitude_deg DOUBLE PRECISION NOT NULL,
    elevation_ft INTEGER,
    continent TEXT NOT NULL,
    iso_country TEXT NOT NULL,
    iso_region TEXT NOT NULL,
    municipality TEXT,
    scheduled_service TEXT NOT NULL,
    icao_code TEXT,
    iata_code TEXT,
    gps_code TEXT,
    local_code TEXT,
    home_link TEXT,
    wikipedia_link TEXT,
    keywords TEXT
);

CREATE INDEX idx_airports_iata_code ON airports (iata_code);
CREATE INDEX idx_airports_iso_country ON airports (iso_country);
CREATE INDEX idx_airports_iso_region ON airports (iso_region);
