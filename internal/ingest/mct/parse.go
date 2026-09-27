package mct

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MCT minutes are planning estimates from Minimum Connection Time
// (https://minimumconnectiontime.com). They are not official OAG or IATA MCT
// and are not airline-, terminal-, or flight-number-specific rules.

const (
	colIATACode                        = "iata_code"
	colName                            = "name"
	colCity                            = "city"
	colCountry                         = "country"
	colICAOCode                        = "icao_code"
	colLatitudeDeg                     = "latitude_deg"
	colLongitudeDeg                    = "longitude_deg"
	colInternational                   = "international"
	colMCTDomesticToDomestic           = "mct_domestic_to_domestic"
	colMCTDomesticToInternational      = "mct_domestic_to_international"
	colMCTInternationalToDomestic      = "mct_international_to_domestic"
	colMCTInternationalToInternational = "mct_international_to_international"
	colMCTInterline                    = "mct_interline"
	colSourceUpdatedOn                 = "source_updated_on"
	maxMCTMinutes                      = 24 * 60
	iataLength                         = 3
	jsonKeyDataset                     = "dataset"
	jsonKeyRows                        = "rows_imported"
)

var (
	errEmptyCatalog = errors.New("mct catalog has no airports")

	airportMCTColumns = []string{
		colIATACode,
		colName,
		colCity,
		colCountry,
		colICAOCode,
		colLatitudeDeg,
		colLongitudeDeg,
		colInternational,
		colMCTDomesticToDomestic,
		colMCTDomesticToInternational,
		colMCTInternationalToDomestic,
		colMCTInternationalToInternational,
		colMCTInterline,
		colSourceUpdatedOn,
	}
)

// Airport is one upstream airport record reduced to the fields this service stores.
type Airport struct {
	Code                         string
	IATACode                     string
	Name                         string
	City                         string
	Country                      string
	ICAO                         string
	Latitude                     *float64
	Longitude                    *float64
	International                *bool
	LastUpdated                  string
	DomesticToDomestic           *int
	DomesticToInternational      *int
	InternationalToDomestic      *int
	InternationalToInternational *int
	Interline                    *int
}

// Page is one /api/airports response.
type Page struct {
	Airports   []Airport
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	HasNext    bool
}

type pagePayload struct {
	Data       []rawAirport `json:"data"`
	Pagination pagination   `json:"pagination"`
}

type pagination struct {
	HasNext    bool `json:"has_next"`
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
}

type rawAirport struct {
	Code                         string   `json:"code"`
	IATACode                     string   `json:"iataCode"`
	Name                         string   `json:"name"`
	City                         string   `json:"city"`
	Country                      string   `json:"country"`
	ICAO                         string   `json:"icao"`
	Latitude                     *float64 `json:"latitude"`
	Longitude                    *float64 `json:"longitude"`
	International                *bool    `json:"international"`
	LastUpdated                  string   `json:"last_updated"`
	DomesticToDomestic           *minutes `json:"mct_domestic_to_domestic"`
	DomesticToInternational      *minutes `json:"mct_domestic_to_international"`
	InternationalToDomestic      *minutes `json:"mct_international_to_domestic"`
	InternationalToInternational *minutes `json:"mct_international_to_international"`
	Interline                    *minutes `json:"mct_interline"`
}

// minutes accepts a JSON integer. A whole-number float is accepted so a
// schema that emits 60.0 still loads. Missing and null stay nil on the pointer.
type minutes int

func (m *minutes) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		return m.set(n)
	}

	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("mct minutes: %w", err)
	}

	if f != math.Trunc(f) {
		return fmt.Errorf("mct minutes: non-integer %s", data)
	}

	if f < 0 || f > maxMCTMinutes {
		return fmt.Errorf("mct minutes %s out of range", data)
	}

	return m.set(int(f))
}

func (m *minutes) set(n int) error {
	if n < 0 || n > maxMCTMinutes {
		return fmt.Errorf("mct minutes %d out of range", n)
	}

	*m = minutes(n)

	return nil
}

func (m *minutes) value() *int {
	if m == nil {
		return nil
	}

	n := int(*m)

	return &n
}

// Columns is the COPY column list for airport_mct.
func Columns() []string {
	return append([]string(nil), airportMCTColumns...)
}

// ParsePage decodes one airports API page. minimal=true responses omit MCT
// fields, so callers must request the full record.
func ParsePage(data []byte) (Page, error) {
	var payload pagePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return Page{}, fmt.Errorf("parse mct page: %w", err)
	}

	airports := make([]Airport, len(payload.Data))
	for i, raw := range payload.Data {
		airports[i] = Airport{
			Code:                         raw.Code,
			IATACode:                     raw.IATACode,
			Name:                         raw.Name,
			City:                         raw.City,
			Country:                      raw.Country,
			ICAO:                         raw.ICAO,
			Latitude:                     raw.Latitude,
			Longitude:                    raw.Longitude,
			International:                raw.International,
			LastUpdated:                  raw.LastUpdated,
			DomesticToDomestic:           raw.DomesticToDomestic.value(),
			DomesticToInternational:      raw.DomesticToInternational.value(),
			InternationalToDomestic:      raw.InternationalToDomestic.value(),
			InternationalToInternational: raw.InternationalToInternational.value(),
			Interline:                    raw.Interline.value(),
		}
	}

	return Page{
		Airports:   airports,
		Page:       payload.Pagination.Page,
		PerPage:    payload.Pagination.PerPage,
		Total:      payload.Pagination.Total,
		TotalPages: payload.Pagination.TotalPages,
		HasNext:    payload.Pagination.HasNext,
	}, nil
}

// ToRows maps airports onto COPY rows keyed by IATA. The first valid code wins.
// Airports without a 3-character IATA code are skipped. An empty result is an
// error so a bad payload cannot full-replace the table with nothing.
func ToRows(airports []Airport) ([]string, [][]string, error) {
	if len(airports) == 0 {
		return nil, nil, errEmptyCatalog
	}

	seen := make(map[string]struct{}, len(airports))
	rows := make([][]string, 0, len(airports))

	for _, airport := range airports {
		code := canonicalIATA(airport.Code, airport.IATACode)
		if code == "" {
			continue
		}

		if _, ok := seen[code]; ok {
			continue
		}

		row, err := airportRow(code, airport)
		if err != nil {
			return nil, nil, err
		}

		seen[code] = struct{}{}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, nil, errEmptyCatalog
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})

	return Columns(), rows, nil
}

func airportRow(code string, airport Airport) ([]string, error) {
	updated, err := formatSourceDate(airport.LastUpdated)
	if err != nil {
		return nil, fmt.Errorf("airport %s: %w", code, err)
	}

	return []string{
		code,
		strings.TrimSpace(airport.Name),
		strings.TrimSpace(airport.City),
		strings.TrimSpace(airport.Country),
		strings.TrimSpace(airport.ICAO),
		formatCoord(airport.Latitude, -90, 90),
		formatCoord(airport.Longitude, -180, 180),
		formatBool(airport.International),
		formatInt(airport.DomesticToDomestic),
		formatInt(airport.DomesticToInternational),
		formatInt(airport.InternationalToDomestic),
		formatInt(airport.InternationalToInternational),
		formatInt(airport.Interline),
		updated,
	}, nil
}

func canonicalIATA(code, iata string) string {
	if v := normalizeIATA(iata); v != "" {
		return v
	}

	return normalizeIATA(code)
}

func normalizeIATA(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if len(raw) != iataLength {
		return ""
	}

	for _, r := range raw {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}

	return raw
}

func formatInt(v *int) string {
	if v == nil {
		return ""
	}

	return strconv.Itoa(*v)
}

func formatBool(v *bool) string {
	if v == nil {
		return ""
	}

	return strconv.FormatBool(*v)
}

func formatCoord(v *float64, minValue, maxValue float64) string {
	if v == nil || *v < minValue || *v > maxValue {
		return ""
	}

	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func formatSourceDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	parsed, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return "", fmt.Errorf("parse last_updated %q: %w", raw, err)
	}

	return parsed.Format(time.DateOnly), nil
}
