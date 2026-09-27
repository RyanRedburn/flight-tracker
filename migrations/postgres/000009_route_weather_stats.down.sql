DROP TABLE IF EXISTS route_weather_category_buckets;

ALTER TABLE weather_observations DROP CONSTRAINT IF EXISTS weather_observations_category_chk;

ALTER TABLE weather_observations
    DROP COLUMN IF EXISTS ceiling_ft,
    DROP COLUMN IF EXISTS category;
