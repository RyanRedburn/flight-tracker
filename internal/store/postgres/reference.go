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

	if err := execCreateJob(ctx, s.db, job); err != nil {
		return nil, err
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
	if len(columns) == 0 {
		return errors.New("columns required")
	}

	table, err := dataset.Table()
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteQuery); err != nil {
		return fmt.Errorf("delete %s rows: %w", table, err)
	}

	if err := replaceTableRows(ctx, tx, table, columns, rows, true); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func isReferenceJobType(jobType string) bool {
	switch jobType {
	case model.JobTypeImportCountries,
		model.JobTypeImportRegions,
		model.JobTypeImportAirports:
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
