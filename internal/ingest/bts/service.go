package bts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/csvparse"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type CSVOpener func(ctx context.Context, year, month int) (csvPath string, cleanup func(), err error)

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

func (s *Service) ImportMonth(ctx context.Context, year, month int) (ImportResult, error) {
	csvPath, cleanup, err := s.openCSVFile(ctx, year, month)
	if err != nil {
		return ImportResult{}, err
	}
	defer cleanup()

	file, err := os.Open(csvPath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open csv: %w", err)
	}
	defer file.Close()

	columns, rows, err := csvparse.Parse(file, DBColumns, csvHeaderToColumn)
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse csv: %w", err)
	}

	if err := s.store.ReplaceFlightPerformanceByMonth(ctx, year, month, columns, rows); err != nil {
		return ImportResult{}, fmt.Errorf("load flights: %w", err)
	}

	return ImportResult{
		Year:         year,
		Month:        month,
		RowsImported: len(rows),
	}, nil
}

func (s *Service) openCSVFile(ctx context.Context, year, month int) (string, func(), error) {
	if s.openCSV != nil {
		path, cleanup, err := s.openCSV(ctx, year, month)

		return path, cleanup, err
	}

	if s.downloader == nil {
		return "", func() {}, errors.New("bts downloader not configured")
	}

	return s.downloader.DownloadCSV(ctx, year, month)
}
