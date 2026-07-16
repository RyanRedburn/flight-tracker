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
    latitude_deg REAL NOT NULL,
    longitude_deg REAL NOT NULL,
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
