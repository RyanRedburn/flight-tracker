# IEM ASOS / METAR hourly observations

Field reference for the `weather_observations` table. Column names match the Iowa
Environmental Mesonet (IEM) ASOS CSV download (`/cgi-bin/request/asos.py`) so
ingest stays a thin map from source → Postgres.

## Background

IEM maintains an archive of automated airport weather observations (ASOS/AWOS),
commonly called METAR data. Programmatic access is documented at:

- Download portal: https://mesonet.agron.iastate.edu/request/download.phtml
- CGI help: https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?help=

This project stores the delay-analysis core subset of those fields. Observations
are requested in **UTC**. Routine METARs often land near `:51` rather than
`:00`; specials (SPECI) add irregular timestamps.

Empty CSV cells are stored as SQL `NULL`. IEM’s default missing token `M` is
avoided by requesting `missing=empty` on download.

## Record layout

Columns appear in table / insert order. `year` and `month` are **not** in the
IEM CSV; they are injected at import time for month-scoped replace.

| Column | Description |
| ------ | ----------- |
| year | Calendar year of the ingest partition (UTC month of `valid`). Used with `month` for delete-and-replace. |
| month | Calendar month of the ingest partition (1–12). |
| station | IEM site identifier (three or four characters). Weather-stations ingest stores the resolved sid on `airport_weather_stations.iem_sid`: BTS/IATA first (`ORD`), then OurAirports FAA `local_code` (`AZA`→`IWA`), then ICAO/ident (`HNL`→`PHNL`). Join observations on `iem_sid`, not the BTS airport code. |
| valid | Observation timestamp (`TIMESTAMPTZ`, stored as UTC). Source CSV values look like `YYYY-MM-DD HH:MM` in the requested timezone. |
| tmpf | Air temperature, typically at 2 meters. Degrees Fahrenheit. |
| dwpf | Dew point temperature, typically at 2 meters. Degrees Fahrenheit. |
| relh | Relative humidity. Percent. |
| drct | Wind direction from true north. Degrees (0–360). |
| sknt | Wind speed. Knots. |
| gust | Wind gust. Knots. |
| vsby | Visibility. Statute miles. |
| skyc1 | Sky coverage at level 1 (e.g. `CLR`, `FEW`, `SCT`, `BKN`, `OVC`, `VV`). |
| skyc2 | Sky coverage at level 2. Same coding as `skyc1`. |
| skyc3 | Sky coverage at level 3. Same coding as `skyc1`. |
| skyl1 | Cloud base height at level 1. Feet above ground. |
| skyl2 | Cloud base height at level 2. Feet above ground. |
| skyl3 | Cloud base height at level 3. Feet above ground. |
| wxcodes | Present weather codes from the METAR (space-separated), e.g. `-SN`, `BR`, `-RA`. |
| p01i | One-hour precipitation for the period ending at the observation (timing of the precip “reset” varies slightly by site). Inches. Trace amounts may appear as a small float (IEM default `0.0001`) depending on download options. |
| alti | Pressure altimeter setting. Inches of mercury (inHg). |
| mslp | Sea-level pressure. Millibars (mb). |
| metar | Unprocessed observation in METAR format (includes ICAO id such as `KORD`). Useful for audit and fields not broken out as columns. |
| ceiling_ft | Lowest ceiling in feet, computed at weather ingest (not in the IEM CSV). Null when the row has not been classified, or when no ceiling layer qualifies. See below. |
| category | One primary weather category, computed at weather ingest (not in the IEM CSV). Null until that load classifies the row. See below. |

## Ceiling and category

`ceiling_ft` is the lowest `BKN`, `OVC`, or `VV` height across `skyc1`/`skyl1`, `skyc2`/`skyl2`, and `skyc3`/`skyl3`, rounded to the nearest foot. `FEW` and `SCT` are not a ceiling. A layer with a missing height is skipped. Heights below 0 ft or above 100000 ft are ignored. When no layer qualifies, `ceiling_ft` is null.

`category` is one value per observation. First match wins:

| Priority | Category | Rule |
| --- | --- | --- |
| 1 | `THUNDER` | A present-weather token contains `TS` (`TS`, `VCTS`, `TSRA`, and any intensity prefix). |
| 2 | `LIFR` | Ceiling under 500 ft, or visibility under 1 SM. |
| 3 | `IFR` | Ceiling under 1000 ft, or visibility under 3 SM. |
| 4 | `MVFR` | Ceiling at or under 3000 ft, or visibility at or under 5 SM. |
| 5 | `WINDY` | The greater of `gust` and `sknt` is at least 25 kt. A missing component is ignored. |
| 6 | `PRECIP` | Otherwise a precip token: `RA`, `DZ`, `SN`, `SG`, `IC`, `PL`, `GR`, `GS`, `UP`, or `FZ` (freezing precip and freezing fog). `BR`-only mist is not precip. |
| 7 | `VFR_FAIR` | Otherwise, when visibility, ceiling, or wind is present. |
| — | `UNKNOWN` | Visibility, ceiling, and wind are all missing, and thunder or precip did not already match. |

Empty `wxcodes` means no phenomena, so fair weather is allowed when the other fields support it. Visibility above 10 SM is clamped to 10 before the thresholds. Negative visibility is treated as missing. Wind speeds outside 0–200 kt are ignored.

Classification runs in Go on each weather load (`internal/ingest/iem/category.go`). A row with a null `category` is skipped when route weather stats are rebuilt.

## Route weather stats

`GET /api/v1/routes/weather-stats` reads `route_weather_category_buckets`. The rollup is per side (origin and destination separately), not a cross of origin category by destination category. On-time uses the route-stats definition: not cancelled, then not diverted, then `arr_del15` < 1.

Each flight is joined to the nearest observation within ±30 minutes. An equal distance keeps the earlier `valid` time.

- **Origin clock:** `crs_dep_time` on `flight_date`, in the origin station IANA timezone (`airport_weather_stations.tzname`).
- **Destination clock:** that departure instant plus `crs_elapsed_time` minutes (CRS block). `crs_arr_time` is not used, so an overnight arrival is not pinned to the departure calendar date.

A side is `UNMATCHED` when the airport has no matched station, the clock cannot be built, no observation falls in the window, or every observation in the window has a null `category`. Unmatched flights are `flights_unmatched` on that side. They are never counted as `VFR_FAIR`. `UNKNOWN` is only the stored category for a matched observation that lacks visibility, ceiling, and wind.

The worker rebuilds the rollup (advisory lock, full replace) after a successful flight-performance load, weather-observation load, or weather-station load. The rebuild reads stored `category` values only. Admins can queue the same rebuild with `POST /internal/rebuild/weather-stats`. Use the POST rebuild when flights or station mappings change and the observations are already classified.

## Notes for analysis

- **Ceiling** for route weather stats is the stored `ceiling_ft` column, not a query-time scan of `skyc*` / `skyl*`.
- **BTS `weather_delay`** is carrier-reported delay-cause minutes, not these METAR measurements; do not treat them as the same signal.
- Joining to BTS flights requires converting local `CRSDepTime` / `DepTime` (`hhmm`) plus `FlightDate` into UTC using the airport IANA timezone, then matching on `station` and `valid`. Timezone comes from `airport_weather_stations.tzname` (copied from `weather_stations.tzname` at mapping ingest), not live IEM GeoJSON. Treat `flight_date + make_time(hh, mm, 0)` as a naive timestamp, then `AT TIME ZONE tzname` to get `timestamptz`:

  `((flight_date + make_time((crs_dep_time / 100), (crs_dep_time % 100), 0)) AT TIME ZONE tzname)`

  Route weather stats use that origin departure clock. The destination clock is scheduled departure plus `crs_elapsed_time`, not `crs_arr_time`. See [Route weather stats](#route-weather-stats) above.
- IEM also offers fields not stored here (ice accretion, peak wind, snow depth, fourth sky layer, heat index/`feel`). Add columns later if needed.
