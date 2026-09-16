package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/uuid"
)

func (s *Store) CreateReferenceIngestJob(ctx context.Context, jobType string) (*model.Job, error) {
	if !isReferenceJobType(jobType) {
		return nil, fmt.Errorf("unsupported reference job type %q", jobType)
	}

	now := time.Now().UTC()
	job := &model.Job{
		ID:        uuid.NewString(),
		Type:      jobType,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, store.QueryAdvisoryXactLock, jobType, 0); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}

	var exists int

	err = tx.QueryRowContext(ctx, store.QueryActiveIngestJob,
		jobType,
		string(model.JobStatusPending),
		string(model.JobStatusRunning),
	).Scan(&exists)
	if err == nil {
		return nil, store.ErrActiveIngestConflict
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if err := execCreateJob(ctx, tx, job); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return job, nil
}

func (s *Store) HasReferenceData(ctx context.Context, dataset store.ReferenceDataset) (bool, error) {
	query, err := hasReferenceDataQuery(dataset)
	if err != nil {
		return false, err
	}

	var exists int

	err = s.db.QueryRowContext(ctx, query).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Store) ReplaceCountries(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceReferenceTable(ctx, store.ReferenceCountries, store.QueryDeleteAllCountries, columns, rows)
}

func (s *Store) ReplaceRegions(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceReferenceTable(ctx, store.ReferenceRegions, store.QueryDeleteAllRegions, columns, rows)
}

func (s *Store) ReplaceAirports(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceReferenceTable(ctx, store.ReferenceAirports, store.QueryDeleteAllAirports, columns, rows)
}

func (s *Store) replaceReferenceTable(
	ctx context.Context,
	dataset store.ReferenceDataset,
	deleteQuery string,
	columns []string,
	rows [][]string,
) error {
	table, err := dataset.Table()
	if err != nil {
		return err
	}

	return s.replaceTable(ctx, deleteQuery, nil, table, columns, rows)
}

func isReferenceJobType(jobType string) bool {
	switch jobType {
	case model.JobTypeImportCountries,
		model.JobTypeImportRegions,
		model.JobTypeImportAirports,
		model.JobTypeImportWeatherStations:
		return true
	default:
		return false
	}
}

func hasReferenceDataQuery(dataset store.ReferenceDataset) (string, error) {
	switch dataset {
	case store.ReferenceCountries:
		return store.QueryHasCountriesData, nil
	case store.ReferenceRegions:
		return store.QueryHasRegionsData, nil
	case store.ReferenceAirports:
		return store.QueryHasAirportsData, nil
	default:
		return "", fmt.Errorf("%w: %q", store.ErrInvalidReferenceDataset, dataset)
	}
}
