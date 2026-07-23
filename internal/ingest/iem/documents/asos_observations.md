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
avoided by requesting `missing=empty` at download time (Phase 3).

## Record layout

Columns appear in table / insert order. `year` and `month` are **not** in the
IEM CSV; they are injected at import time for month-scoped replace.

| Column | Description |
| ------ | ----------- |
| year | Calendar year of the ingest partition (UTC month of `valid`). Used with `month` for delete-and-replace. |
| month | Calendar month of the ingest partition (1–12). |
| station | IEM site identifier (three or four characters). For most CONUS airports this matches the FAA/IATA code used by BTS `origin`/`dest` (e.g. `ORD`, not `KORD`). Alaska, Hawaii, and territories may differ. |
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

## Notes for analysis

- **Ceiling** is not a stored column. Derive it from `skyc*` / `skyl*` (typically the lowest `BKN`/`OVC`/`VV` layer).
- **BTS `weather_delay`** is carrier-reported delay-cause minutes, not these METAR measurements; do not treat them as the same signal.
- Joining to BTS flights requires converting local `CRSDepTime` / `DepTime` (`hhmm`) plus `FlightDate` into UTC using the airport timezone before matching on `station` and `valid`.
- IEM also offers fields not stored here (ice accretion, peak wind, snow depth, fourth sky layer, heat index/`feel`). Add columns later if needed.
