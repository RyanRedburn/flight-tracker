package iem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/csvparse"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// CSVOpener opens a local CSV for tests. When set, it replaces the live downloader.
type CSVOpener func(ctx context.Context, year, month int, stations []string) (csvPath string, cleanup func(), err error)

type Service struct {
	store      store.Store
	downloader *Downloader
	catalog    *NetworkCatalog
	openCSV    CSVOpener
}

func NewService(s store.Store, downloader *Downloader) *Service {
	return &Service{
		store:      s,
		downloader: downloader,
	}
}

func (s *Service) WithCSVOpener(opener CSVOpener) *Service {
	s.openCSV = opener

	return s
}

func (s *Service) WithCatalog(catalog *NetworkCatalog) *Service {
	s.catalog = catalog

	return s
}

type ImportResult struct {
	Year         int `json:"year"`
	Month        int `json:"month"`
	RowsImported int `json:"rows_imported"`
}

func (r ImportResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		colYear:      r.Year,
		jsonKeyMonth: r.Month,
		jsonKeyRows:  r.RowsImported,
	})
}

type StationsImportResult struct {
	StationsLoaded    int
	MatchedAirports   int
	UnmatchedAirports int
	Unmatched         []string
}

func (r StationsImportResult) MarshalJSON() ([]byte, error) {
	payload := map[string]any{
		jsonKeyStationsLoaded:    r.StationsLoaded,
		jsonKeyMatchedAirports:   r.MatchedAirports,
		jsonKeyUnmatchedAirports: r.UnmatchedAirports,
	}

	if len(r.Unmatched) > 0 {
		payload[jsonKeyUnmatched] = r.Unmatched
	}

	return json.Marshal(payload)
}

func (s *Service) ImportMonth(ctx context.Context, year, month int, stations []string) (ImportResult, error) {
	if len(stations) == 0 {
		return ImportResult{}, ErrEmptyStations
	}

	csvPath, cleanup, err := s.openCSVFile(ctx, year, month, stations)
	if err != nil {
		return ImportResult{}, err
	}
	defer cleanup()

	file, err := os.Open(csvPath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open csv: %w", err)
	}
	defer file.Close()

	columns, rows, err := csvparse.Parse(file, ObservationColumns, csvHeaderToColumn)
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse csv: %w", err)
	}

	dbColumns, dbRows, err := withPartitionKeys(year, month, columns, rows)
	if err != nil {
		return ImportResult{}, err
	}

	dbRows, err = dedupWeatherRows(dbColumns, dbRows)
	if err != nil {
		return ImportResult{}, err
	}

	if err := s.store.ReplaceWeatherObservationsByMonth(ctx, year, month, dbColumns, dbRows); err != nil {
		return ImportResult{}, fmt.Errorf("load weather: %w", err)
	}

	return ImportResult{
		Year:         year,
		Month:        month,
		RowsImported: len(dbRows),
	}, nil
}

func (s *Service) ImportStations(ctx context.Context) (StationsImportResult, error) {
	if s.catalog == nil {
		return StationsImportResult{}, errors.New("iem network catalog not configured")
	}

	catalog, err := s.catalog.LoadStations(ctx)
	if err != nil {
		return StationsImportResult{}, err
	}

	flightCodes, err := s.store.DistinctFlightAirportCodes(ctx)
	if err != nil {
		return StationsImportResult{}, fmt.Errorf("list flight airports: %w", err)
	}

	refs, err := s.store.ListAirportIdentifiersByIATA(ctx, flightCodes)
	if err != nil {
		return StationsImportResult{}, fmt.Errorf("list airport identifiers: %w", err)
	}

	if len(flightCodes) > 0 && len(refs) == 0 {
		slog.Default().Warn("weather stations ingest: no OurAirports identifiers for flight codes; matching IEM sid to BTS code only",
			"flight_airport_count", len(flightCodes),
		)
	}

	mapping := mapFlightAirports(flightCodes, catalog, refs)
	stationRows := encodeWeatherStations(catalog)
	mappingRows := encodeAirportWeatherStations(mapping, time.Now().UTC())

	if err := s.store.ReplaceWeatherStations(
		ctx,
		append([]string(nil), weatherStationColumns...),
		stationRows,
		append([]string(nil), airportWeatherStationColumns...),
		mappingRows,
	); err != nil {
		return StationsImportResult{}, fmt.Errorf("load weather stations: %w", err)
	}

	unmatched := make([]string, 0)
	matched := 0

	for _, row := range mapping {
		if row.Matched {
			matched++
			continue
		}

		unmatched = append(unmatched, row.AirportCode)
	}

	return StationsImportResult{
		StationsLoaded:    len(catalog),
		MatchedAirports:   matched,
		UnmatchedAirports: len(unmatched),
		Unmatched:         unmatched,
	}, nil
}

func (s *Service) openCSVFile(ctx context.Context, year, month int, stations []string) (string, func(), error) {
	if s.openCSV != nil {
		return s.openCSV(ctx, year, month, stations)
	}

	if s.downloader == nil {
		return "", func() {}, errors.New("iem downloader not configured")
	}

	return s.downloader.DownloadCSV(ctx, year, month, stations)
}

func withPartitionKeys(year, month int, columns []string, rows [][]string) ([]string, [][]string, error) {
	validIdx := -1

	for i, col := range columns {
		if col == colValid {
			validIdx = i
			break
		}
	}

	if validIdx < 0 {
		return nil, nil, errors.New("valid column required")
	}

	dbColumns := make([]string, 0, len(columns)+2)
	dbColumns = append(dbColumns, colYear, colMonth)
	dbColumns = append(dbColumns, columns...)

	yearStr := strconv.Itoa(year)
	monthStr := strconv.Itoa(month)
	dbRows := make([][]string, len(rows))

	for i, row := range rows {
		normalized := make([]string, len(row))
		copy(normalized, row)

		if normalized[validIdx] != "" {
			utc, err := parseIEMValidUTC(normalized[validIdx])
			if err != nil {
				return nil, nil, fmt.Errorf("row %d valid %q: %w", i+1, normalized[validIdx], err)
			}

			normalized[validIdx] = utc.Format(time.RFC3339)
		}

		dbRows[i] = make([]string, 0, len(normalized)+2)
		dbRows[i] = append(dbRows[i], yearStr, monthStr)
		dbRows[i] = append(dbRows[i], normalized...)
	}

	return dbColumns, dbRows, nil
}

func dedupWeatherRows(columns []string, rows [][]string) ([][]string, error) {
	stationIdx, validIdx := -1, -1

	for i, col := range columns {
		switch col {
		case colStation:
			stationIdx = i
		case colValid:
			validIdx = i
		}
	}

	if stationIdx < 0 || validIdx < 0 {
		return nil, errors.New("station and valid columns required")
	}

	type key struct {
		station string
		valid   string
	}

	last := make(map[key]int, len(rows))

	for i, row := range rows {
		if len(row) <= stationIdx || len(row) <= validIdx {
			return nil, fmt.Errorf("row %d width %d does not include station/valid", i+1, len(row))
		}

		last[key{station: row[stationIdx], valid: row[validIdx]}] = i
	}

	if len(last) == len(rows) {
		return rows, nil
	}

	out := make([][]string, 0, len(last))

	for i, row := range rows {
		k := key{station: row[stationIdx], valid: row[validIdx]}
		if last[k] != i {
			continue
		}

		out = append(out, row)
	}

	return out, nil
}

// parseIEMValidUTC parses IEM "YYYY-MM-DD HH:MM" (or with seconds) as UTC.
func parseIEMValidUTC(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)

	layouts := []string{
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t, nil
		}
	}

	return time.Time{}, errors.New("unsupported timestamp")
}
