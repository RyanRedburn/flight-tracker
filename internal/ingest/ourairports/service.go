package ourairports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type CSVOpener func(ctx context.Context, dataset store.OurAirportsDataset) (csvPath string, cleanup func(), err error)

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
	Dataset      store.OurAirportsDataset `json:"dataset"`
	RowsImported int                      `json:"rows_imported"`
}

func (r ImportResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		jsonKeyDataset: string(r.Dataset),
		jsonKeyRows:    r.RowsImported,
	})
}

func (s *Service) Import(ctx context.Context, dataset store.OurAirportsDataset) (ImportResult, error) {
	csvPath, cleanup, err := s.openCSVFile(ctx, dataset)
	if err != nil {
		return ImportResult{}, err
	}
	defer cleanup()

	file, err := os.Open(csvPath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open csv: %w", err)
	}
	defer file.Close()

	columns, rows, err := ParseCSV(file, dataset)
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse csv: %w", err)
	}

	if err := s.replaceDataset(ctx, dataset, columns, rows); err != nil {
		return ImportResult{}, fmt.Errorf("load %s: %w", dataset, err)
	}

	return ImportResult{
		Dataset:      dataset,
		RowsImported: len(rows),
	}, nil
}

func (s *Service) openCSVFile(ctx context.Context, dataset store.OurAirportsDataset) (string, func(), error) {
	if s.openCSV != nil {
		path, cleanup, err := s.openCSV(ctx, dataset)

		return path, cleanup, err
	}

	if s.downloader == nil {
		return "", func() {}, errors.New("ourairports downloader not configured")
	}

	return s.downloader.DownloadCSV(ctx, dataset)
}

func (s *Service) replaceDataset(ctx context.Context, dataset store.OurAirportsDataset, columns []string, rows [][]string) error {
	switch dataset {
	case store.OurAirportsCountries:
		return s.store.ReplaceOurAirportsCountries(ctx, columns, rows)
	case store.OurAirportsRegions:
		return s.store.ReplaceOurAirportsRegions(ctx, columns, rows)
	case store.OurAirportsAirports:
		return s.store.ReplaceOurAirportsAirports(ctx, columns, rows)
	default:
		return fmt.Errorf("%w: %q", store.ErrInvalidOurAirportsDataset, dataset)
	}
}
