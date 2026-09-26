-- METAR category on each observation, plus per-side route weather rollups.
-- ceiling_ft is the lowest BKN/OVC/VV height across skyc1–skyc3.
-- category is one primary value (first match): THUNDER, LIFR, IFR, MVFR,
-- WINDY, PRECIP, VFR_FAIR, UNKNOWN. Rules match internal/ingest/iem/category.go.
-- New loads classify in Go before COPY. This migration backfills rows already stored.
--
-- route_weather_category_buckets is the read model for GET /api/v1/routes/weather-stats.
-- Grain is per side (origin or dest), not an origin-category × dest-category matrix.
-- carrier '' is a missing marketing carrier. flight_number -1 is a missing flight number.
-- UNMATCHED is a rollup bucket only (no observation in the ±30 minute window).

ALTER TABLE weather_observations
    ADD COLUMN ceiling_ft INTEGER,
    ADD COLUMN category TEXT;

CREATE FUNCTION weather_obs_ceiling_ft(
    skyc1 text,
    skyl1 double precision,
    skyc2 text,
    skyl2 double precision,
    skyc3 text,
    skyl3 double precision
) RETURNS integer
LANGUAGE sql
STABLE
AS $fn$
    SELECT MIN(height)::integer
    FROM (
        SELECT CASE
            WHEN upper(btrim(skyc1)) IN ('BKN', 'OVC', 'VV')
                AND skyl1 IS NOT NULL
                AND skyl1 >= 0
                AND skyl1 <= 100000
            THEN round(skyl1)::integer
        END AS height
        UNION ALL
        SELECT CASE
            WHEN upper(btrim(skyc2)) IN ('BKN', 'OVC', 'VV')
                AND skyl2 IS NOT NULL
                AND skyl2 >= 0
                AND skyl2 <= 100000
            THEN round(skyl2)::integer
        END
        UNION ALL
        SELECT CASE
            WHEN upper(btrim(skyc3)) IN ('BKN', 'OVC', 'VV')
                AND skyl3 IS NOT NULL
                AND skyl3 >= 0
                AND skyl3 <= 100000
            THEN round(skyl3)::integer
        END
    ) layers
    WHERE height IS NOT NULL
$fn$;

CREATE FUNCTION weather_obs_category(
    vsby double precision,
    sknt double precision,
    gust double precision,
    skyc1 text,
    skyl1 double precision,
    skyc2 text,
    skyl2 double precision,
    skyc3 text,
    skyl3 double precision,
    wxcodes text
) RETURNS text
LANGUAGE plpgsql
STABLE
AS $fn$
DECLARE
    ceiling integer;
    vis double precision;
    wind double precision;
    tok text;
    thunder boolean := false;
    precip boolean := false;
BEGIN
    ceiling := weather_obs_ceiling_ft(skyc1, skyl1, skyc2, skyl2, skyc3, skyl3);

    IF vsby IS NOT NULL AND vsby >= 0 THEN
        vis := LEAST(vsby, 10);
    END IF;

    IF sknt IS NOT NULL AND sknt >= 0 AND sknt <= 200 THEN
        wind := sknt;
    END IF;

    IF gust IS NOT NULL AND gust >= 0 AND gust <= 200 THEN
        IF wind IS NULL OR gust > wind THEN
            wind := gust;
        END IF;
    END IF;

    IF wxcodes IS NOT NULL AND btrim(wxcodes) <> '' THEN
        FOR tok IN
            SELECT ltrim(upper(btrim(part)), '+-')
            FROM regexp_split_to_table(wxcodes, E'\\s+') AS part
        LOOP
            IF tok IS NULL OR tok = '' THEN
                CONTINUE;
            END IF;

            IF position('TS' IN tok) > 0 THEN
                thunder := true;
            END IF;

            IF position('FZ' IN tok) > 0
                OR position('RA' IN tok) > 0
                OR position('DZ' IN tok) > 0
                OR position('SN' IN tok) > 0
                OR position('SG' IN tok) > 0
                OR position('IC' IN tok) > 0
                OR position('PL' IN tok) > 0
                OR position('GR' IN tok) > 0
                OR position('GS' IN tok) > 0
                OR position('UP' IN tok) > 0
            THEN
                precip := true;
            END IF;
        END LOOP;
    END IF;

    IF thunder THEN
        RETURN 'THUNDER';
    END IF;

    IF (ceiling IS NOT NULL AND ceiling < 500) OR (vis IS NOT NULL AND vis < 1) THEN
        RETURN 'LIFR';
    END IF;

    IF (ceiling IS NOT NULL AND ceiling < 1000) OR (vis IS NOT NULL AND vis < 3) THEN
        RETURN 'IFR';
    END IF;

    IF (ceiling IS NOT NULL AND ceiling <= 3000) OR (vis IS NOT NULL AND vis <= 5) THEN
        RETURN 'MVFR';
    END IF;

    IF wind IS NOT NULL AND wind >= 25 THEN
        RETURN 'WINDY';
    END IF;

    IF precip THEN
        RETURN 'PRECIP';
    END IF;

    IF vis IS NULL AND ceiling IS NULL AND wind IS NULL THEN
        RETURN 'UNKNOWN';
    END IF;

    RETURN 'VFR_FAIR';
END;
$fn$;

UPDATE weather_observations
SET
    ceiling_ft = weather_obs_ceiling_ft(skyc1, skyl1, skyc2, skyl2, skyc3, skyl3),
    category = weather_obs_category(vsby, sknt, gust, skyc1, skyl1, skyc2, skyl2, skyc3, skyl3, wxcodes)
WHERE category IS NULL;

ALTER TABLE weather_observations
    ALTER COLUMN category SET NOT NULL;

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
