-- airport_mct holds planning estimates from Minimum Connection Time
-- (https://minimumconnectiontime.com). These figures are compiled from public
-- sources. They are not official OAG or IATA MCT, and they are not
-- airline-, terminal-, or flight-number-specific rules.
CREATE TABLE airport_mct (
    iata_code TEXT PRIMARY KEY,
    name TEXT,
    city TEXT,
    country TEXT,
    icao_code TEXT,
    latitude_deg DOUBLE PRECISION,
    longitude_deg DOUBLE PRECISION,
    international BOOLEAN,
    mct_domestic_to_domestic INTEGER,
    mct_domestic_to_international INTEGER,
    mct_international_to_domestic INTEGER,
    mct_international_to_international INTEGER,
    mct_interline INTEGER,
    source_updated_on DATE,
    CONSTRAINT airport_mct_iata_code_chk CHECK (iata_code ~ '^[A-Z0-9]{3}$'),
    CONSTRAINT airport_mct_latitude_chk CHECK (latitude_deg IS NULL OR latitude_deg BETWEEN -90 AND 90),
    CONSTRAINT airport_mct_longitude_chk CHECK (longitude_deg IS NULL OR longitude_deg BETWEEN -180 AND 180),
    CONSTRAINT airport_mct_minutes_chk CHECK (
        (mct_domestic_to_domestic IS NULL OR mct_domestic_to_domestic >= 0)
        AND (mct_domestic_to_international IS NULL OR mct_domestic_to_international >= 0)
        AND (mct_international_to_domestic IS NULL OR mct_international_to_domestic >= 0)
        AND (mct_international_to_international IS NULL OR mct_international_to_international >= 0)
        AND (mct_interline IS NULL OR mct_interline >= 0)
    )
);
