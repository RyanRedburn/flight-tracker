# Marketing Carrier On-Time Performance (Beginning January 2018)

Each monthly TranStats zip includes `readme.html` with the full column list. This service stores those columns. Stats, outlook, and weather joins use the ones below. The CSV header `DayofMonth` is stored as `day_of_month`.

| Column | Meaning |
| --- | --- |
| `year`, `month` | Calendar year and month of the flight. Also the ingest partition. |
| `flight_date` | Flight date (local). |
| `day_of_week` | 1 = Monday through 7 = Sunday. |
| `iata_code_marketing_airline` | Marketing carrier IATA code. This is the API `carrier`. The same code can belong to different airlines in different years. |
| `flight_number_marketing_airline` | Marketing flight number. |
| `origin`, `dest` | Origin and destination airport codes. |
| `origin_state`, `dest_state` | State codes for those airports. |
| `crs_dep_time` | Scheduled departure, local time as `hhmm` (`0700` is 7:00). Not minutes past midnight. |
| `crs_elapsed_time` | Scheduled gate-to-gate block, minutes. |
| `cancelled` | `1` = cancelled. |
| `diverted` | `1` = diverted. |
| `arr_del15` | `1` = arrival at least 15 minutes late. |
| `arr_delay_minutes`, `dep_delay_minutes` | Minutes late. An early flight is `0` here. The signed `arr_delay` and `dep_delay` columns can be negative; stats use the minutes columns. |
| `carrier_delay`, `weather_delay`, `nas_delay`, `security_delay`, `late_aircraft_delay` | Carrier-reported delay-cause minutes. `weather_delay` is that report, not a METAR. |
| `div1_airport` … `div5_airport` | Diverted-airport codes, in landing order. |
| `duplicate` | `Y` if the row is a Form-3A codeshare swap ("Duplicate flag marked Y if the flight is swapped based on Form-3A data"). See `duplicate_column.md`. |

On time in this service means not cancelled, then not diverted, then `arr_del15` < 1.

Source: TranStats Marketing Carrier On-Time Performance (Beginning January 2018), `readme.html` inside the month zip.
