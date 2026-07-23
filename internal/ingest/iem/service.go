package iem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	if err := s.store.ReplaceWeatherObservationsByMonth(ctx, year, month, dbColumns, dbRows); err != nil {
		return ImportResult{}, fmt.Errorf("load weather: %w", err)
	}

	return ImportResult{
		Year:         year,
		Month:        month,
		RowsImported: len(dbRows),
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

	return time.Time{}, fmt.Errorf("unsupported timestamp")
}
