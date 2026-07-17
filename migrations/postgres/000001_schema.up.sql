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
    year TEXT,
    quarter TEXT,
    month TEXT,
    day_of_month TEXT,
    day_of_week TEXT,
    flight_date TEXT,
    marketing_airline_network TEXT,
    operated_or_branded_code_share_partners TEXT,
    dot_id_marketing_airline TEXT,
    iata_code_marketing_airline TEXT,
    flight_number_marketing_airline TEXT,
    originally_scheduled_code_share_airline TEXT,
    dot_id_originally_scheduled_code_share_airline TEXT,
    iata_code_originally_scheduled_code_share_airline TEXT,
    flight_num_originally_scheduled_code_share_airline TEXT,
    operating_airline TEXT,
    dot_id_operating_airline TEXT,
    iata_code_operating_airline TEXT,
    tail_number TEXT,
    flight_number_operating_airline TEXT,
    origin_airport_id TEXT,
    origin_airport_seq_id TEXT,
    origin_city_market_id TEXT,
    origin TEXT,
    origin_city_name TEXT,
    origin_state TEXT,
    origin_state_fips TEXT,
    origin_state_name TEXT,
    origin_wac TEXT,
    dest_airport_id TEXT,
    dest_airport_seq_id TEXT,
    dest_city_market_id TEXT,
    dest TEXT,
    dest_city_name TEXT,
    dest_state TEXT,
    dest_state_fips TEXT,
    dest_state_name TEXT,
    dest_wac TEXT,
    crs_dep_time TEXT,
    dep_time TEXT,
    dep_delay TEXT,
    dep_delay_minutes TEXT,
    dep_del15 TEXT,
    departure_delay_groups TEXT,
    dep_time_blk TEXT,
    taxi_out TEXT,
    wheels_off TEXT,
    wheels_on TEXT,
    taxi_in TEXT,
    crs_arr_time TEXT,
    arr_time TEXT,
    arr_delay TEXT,
    arr_delay_minutes TEXT,
    arr_del15 TEXT,
    arrival_delay_groups TEXT,
    arr_time_blk TEXT,
    cancelled TEXT,
    cancellation_code TEXT,
    diverted TEXT,
    crs_elapsed_time TEXT,
    actual_elapsed_time TEXT,
    air_time TEXT,
    flights TEXT,
    distance TEXT,
    distance_group TEXT,
    carrier_delay TEXT,
    weather_delay TEXT,
    nas_delay TEXT,
    security_delay TEXT,
    late_aircraft_delay TEXT,
    first_dep_time TEXT,
    total_add_g_time TEXT,
    longest_add_g_time TEXT,
    div_airport_landings TEXT,
    div_reached_dest TEXT,
    div_actual_elapsed_time TEXT,
    div_arr_delay TEXT,
    div_distance TEXT,
    div1_airport TEXT,
    div1_airport_id TEXT,
    div1_airport_seq_id TEXT,
    div1_wheels_on TEXT,
    div1_total_g_time TEXT,
    div1_longest_g_time TEXT,
    div1_wheels_off TEXT,
    div1_tail_num TEXT,
    div2_airport TEXT,
    div2_airport_id TEXT,
    div2_airport_seq_id TEXT,
    div2_wheels_on TEXT,
    div2_total_g_time TEXT,
    div2_longest_g_time TEXT,
    div2_wheels_off TEXT,
    div2_tail_num TEXT,
    div3_airport TEXT,
    div3_airport_id TEXT,
    div3_airport_seq_id TEXT,
    div3_wheels_on TEXT,
    div3_total_g_time TEXT,
    div3_longest_g_time TEXT,
    div3_wheels_off TEXT,
    div3_tail_num TEXT,
    div4_airport TEXT,
    div4_airport_id TEXT,
    div4_airport_seq_id TEXT,
    div4_wheels_on TEXT,
    div4_total_g_time TEXT,
    div4_longest_g_time TEXT,
    div4_wheels_off TEXT,
    div4_tail_num TEXT,
    div5_airport TEXT,
    div5_airport_id TEXT,
    div5_airport_seq_id TEXT,
    div5_wheels_on TEXT,
    div5_total_g_time TEXT,
    div5_longest_g_time TEXT,
    div5_wheels_off TEXT,
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
