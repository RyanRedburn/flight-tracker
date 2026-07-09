package mem

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/uuid"
)

func (s *Store) CreateOurAirportsIngestJob(ctx context.Context, jobType string) (*model.Job, error) {
	if !isOurAirportsJobType(jobType) {
		return nil, fmt.Errorf("unsupported ourairports job type %q", jobType)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	job := &model.Job{
		ID:        uuid.NewString(),
		Type:      jobType,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.jobs[job.ID] = cloneJob(job)

	return cloneJob(job), nil
}

func (s *Store) HasOurAirportsData(ctx context.Context, dataset store.OurAirportsDataset) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.ourAirportsRowsLocked(dataset)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

func (s *Store) ReplaceOurAirportsCountries(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceOurAirportsTable(store.OurAirportsCountries, columns, rows)
}

func (s *Store) ReplaceOurAirportsRegions(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceOurAirportsTable(store.OurAirportsRegions, columns, rows)
}

func (s *Store) ReplaceOurAirportsAirports(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceOurAirportsTable(store.OurAirportsAirports, columns, rows)
}

func (s *Store) replaceOurAirportsTable(
	dataset store.OurAirportsDataset,
	columns []string,
	rows [][]string,
) error {
	if len(columns) == 0 {
		return errors.New("columns required")
	}

	for _, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("row width %d does not match columns %d", len(row), len(columns))
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ourAirports == nil {
		s.ourAirports = make(map[store.OurAirportsDataset]ourAirportsTable)
	}

	clonedRows := make([][]string, len(rows))
	for i, row := range rows {
		clonedRows[i] = append([]string(nil), row...)
	}

	s.ourAirports[dataset] = ourAirportsTable{
		columns: append([]string(nil), columns...),
		rows:    clonedRows,
	}

	return nil
}

func (s *Store) ourAirportsRowsLocked(dataset store.OurAirportsDataset) ([][]string, error) {
	if _, err := dataset.Table(); err != nil {
		return nil, err
	}

	if s.ourAirports == nil {
		return nil, nil
	}

	table, ok := s.ourAirports[dataset]
	if !ok {
		return nil, nil
	}

	return table.rows, nil
}

type ourAirportsTable struct {
	columns []string
	rows    [][]string
}

func isOurAirportsJobType(jobType string) bool {
	switch jobType {
	case model.JobTypeImportOurAirportsCountries,
		model.JobTypeImportOurAirportsRegions,
		model.JobTypeImportOurAirportsAirports:
		return true
	default:
		return false
	}
}
