// Package mct loads airport minimum connection times from the public
// Minimum Connection Time API (https://minimumconnectiontime.com).
package mct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// Fetcher returns the full airport catalog. Tests inject this so imports do
// not call the live API.
type Fetcher func(ctx context.Context) ([]Airport, error)

type Service struct {
	store store.Store
	fetch Fetcher
}

// ImportResult is the JSON stored on a completed import_airport_mct job.
type ImportResult struct {
	Dataset      store.ReferenceDataset `json:"dataset"`
	RowsImported int                    `json:"rows_imported"`
}

func NewService(s store.Store, downloader *Downloader) *Service {
	svc := &Service{store: s}
	if downloader != nil {
		svc.fetch = downloader.FetchAirports
	}

	return svc
}

func (s *Service) WithFetcher(fetch Fetcher) *Service {
	s.fetch = fetch

	return s
}

func (r ImportResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		jsonKeyDataset: string(r.Dataset),
		jsonKeyRows:    r.RowsImported,
	})
}

// Import downloads the catalog and full-replaces airport_mct. force is not
// consulted here; it only bypasses the exists-data check when the job is queued.
func (s *Service) Import(ctx context.Context) (ImportResult, error) {
	airports, err := s.openCatalog(ctx)
	if err != nil {
		return ImportResult{}, err
	}

	columns, rows, err := ToRows(airports)
	if err != nil {
		return ImportResult{}, err
	}

	if err := s.store.ReplaceAirportMCT(ctx, columns, rows); err != nil {
		return ImportResult{}, fmt.Errorf("load airport mct: %w", err)
	}

	return ImportResult{
		Dataset:      store.ReferenceAirportMCT,
		RowsImported: len(rows),
	}, nil
}

func (s *Service) openCatalog(ctx context.Context) ([]Airport, error) {
	if s.fetch == nil {
		return nil, errors.New("mct downloader not configured")
	}

	return s.fetch(ctx)
}
