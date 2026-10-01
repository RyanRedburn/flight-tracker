# IEM ASOS / METAR hourly observations

Observations come from the IEM ASOS CSV (`/cgi-bin/request/asos.py`). Stored column names match that download. `year` and `month` are not in the CSV; import injects them so a month can be replaced. Empty cells are SQL `NULL` (`missing=empty` on download). Routine METARs often land near `:51` rather than `:00`.

- Download portal: https://mesonet.agron.iastate.edu/request/download.phtml
- CGI help: https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?help=

Join observations on `airport_weather_stations.iem_sid`, not the BTS airport code. Weather-stations ingest stores the resolved sid: BTS/IATA first (`ORD`), then OurAirports FAA `local_code` (`AZA`→`IWA`), then ICAO/ident (`HNL`→`PHNL`).

## Columns

| Column | Meaning |
| --- | --- |
| `year`, `month` | Ingest partition (UTC month of `valid`). Not in the IEM CSV. |
| `station` | IEM site id (3–4 characters). The resolved sid, not the BTS airport code. |
| `valid` | Observation time, stored as UTC. CSV values look like `YYYY-MM-DD HH:MM` in the requested timezone. |
| `tmpf` | Air temperature, °F. |
| `dwpf` | Dew point, °F. |
| `relh` | Relative humidity, percent. |
| `drct` | Wind direction, degrees from true north (0–360). |
| `sknt` | Wind speed, knots. |
| `gust` | Wind gust, knots. |
| `vsby` | Visibility, statute miles. |
| `skyc1`, `skyc2`, `skyc3` | Sky cover at levels 1–3: `CLR` clear, `FEW` few, `SCT` scattered, `BKN` broken, `OVC` overcast, `VV` vertical visibility (obscured sky). |
| `skyl1`, `skyl2`, `skyl3` | Height of that level, feet above ground. |
| `wxcodes` | Present-weather tokens from the METAR, space-separated (`-SN`, `BR`, `TSRA`). |
| `p01i` | Precipitation in the hour ending at the observation, inches. A trace may be a small float (IEM often uses `0.0001`). |
| `alti` | Altimeter setting, inches of mercury. |
| `mslp` | Sea-level pressure, millibars. |
| `metar` | Raw METAR text, including the ICAO id (`KORD`). |
| `ceiling_ft`, `category` | Computed at ingest. Not in the IEM CSV. See below. |

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

The worker rebuilds the rollup (advisory lock, full replace) after a successful flight-performance load, weather-observation load, or weather-station load. The rebuild reads stored `category` values only. Admins can queue the same rebuild with `POST /api/v1/rebuild/weather-stats`. Use the POST rebuild when flights or station mappings change and the observations are already classified.

Local `hhmm` on `flight_date` becomes a timestamp with:

```text
((flight_date + make_time((crs_dep_time / 100), (crs_dep_time % 100), 0)) AT TIME ZONE tzname)
```

`tzname` is copied from `weather_stations` at mapping ingest. BTS `weather_delay` is carrier-reported delay-cause minutes, not these METAR measurements.
