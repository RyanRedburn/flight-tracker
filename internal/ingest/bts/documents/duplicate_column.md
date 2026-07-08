# `Duplicate` column (Marketing Carrier On-Time Performance)

The TranStats field description is:

> Duplicate flag marked Y if the flight is swapped based on Form-3A data

Despite the name, **`Duplicate` does not mean the CSV row is a literal duplicate**. It marks records reported via **Form 3A** after a **codeshare swap**.

## Short definition

| Value | Meaning |
| ----- | ------- |
| `N` | Ordinary on-time record |
| `Y` | Codeshare-swap record filed on Form 3A (original schedule identity stitched to the substitute operation) |

This field appears in Marketing Carrier On-Time Performance data beginning January 2018 (see `gte_2018_readme.md`).

## Background: Form 3A and codeshare swaps

Starting with the July 2017 OAEP enforcement policy (BTS Technical Directive No. 27, effective 1 Jan 2018), a **marketing carrier** may report a codeshare substitution instead of separately filing a cancellation for the original flight and an on-time record for a substitute.

**Form 3A** (*On-Time Performance Data for Codeshare Flights (Long)*) is used when all of the following hold:

1. The originally scheduled flight was a U.S. codeshare held out with only one U.S. carrier designator.
2. There was a flight substitution (**codeshare swap**).
3. The reporting (marketing) carrier marketed the original codeshare but **operated neither** the original nor the substitute.
4. The substitute was an **extra-section** flight, **or** the carrier that operated it is **not** a Part 234 reporting carrier.

Related form: **Form 3B** covers similar swaps when the substitute is *not* an extra-section and *is* operated by a reporting carrier. The official `Duplicate` description ties the flag specifically to **Form-3A** data.

## Worked example (from Technical Directive No. 27)

- Marketing carrier **XX** sells flight **XX1234**, scheduled to be operated by codeshare partner **YY**.
- **YY** cannot operate (e.g. mechanical). Partner **ZZ** flies the market instead as **XX5678**.
- Flight 1234 was cancelled before leaving the gate.

Under the enforcement-policy option, **XX** does not report cancelled XX1234. It reports the substitute as Form 3A, listing:

- originally scheduled operator / flight: **YY** / **1234**
- actual operator / flight: **ZZ** / **5678**

That Form 3A submission is what surfaces in the downloadable table with `Duplicate = Y`.

## How the hybrid row is constructed

On Form 3A:

- Schedule and delay inputs tied to OAG/CRS departure and arrival times use the **originally scheduled, not-operated** flight.
- Operational fields (actual times, tail number, etc.) use the **substitute flight that actually operated**.

Post-2017 Marketing Carrier columns reflect that split, including:

- `Originally_Scheduled_Code_Share_Airline` (and related ID / IATA / flight-number fields)
- marketing vs operating airline and flight-number fields

## Practical notes for ingest / analysis

- Treat `Y` as **“Form-3A codeshare-swap hybrid”**, not as “drop because this key appears twice.”
- These rows are uncommon relative to `N` records; absence of `Y` in a small sample month is expected.
- Whether to include or exclude `Y` rows depends on the analysis. Including them keeps passenger-facing schedule identity aligned with substitute operations; excluding them may be preferable if counting physical operations reported primarily by operating carriers.

## Sources

- Field list: TranStats Marketing Carrier On-Time Performance (`Duplicate` description matches `gte_2018_readme.md`)
- Form layout and footnotes: [Form 3A](https://esubmit.rita.dot.gov/On-Time-Form3A.aspx)
- Reporting rules and swap example: [Technical Directive No. 27 — On-Time Reporting (effective Jan 1, 2018)](https://www.bts.gov/topics/airlines-and-airports/%E2%80%A2-number-27-technical-directive-time-reporting-effective-jan-1-2018)
