package sqlite

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

func (s *Store) CreateOurAirportsIngestJob(ctx context.Context, jobType string) (*model.Job, error) {
	if !isOurAirportsJobType(jobType) {
		return nil, fmt.Errorf("unsupported ourairports job type %q", jobType)
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

func (s *Store) HasOurAirportsData(ctx context.Context, dataset store.OurAirportsDataset) (bool, error) {
	query, err := hasOurAirportsDataQuery(dataset)
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

func (s *Store) ReplaceOurAirportsCountries(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceOurAirportsTable(ctx, store.OurAirportsCountries, store.QueryDeleteAllCountries, columns, rows)
}

func (s *Store) ReplaceOurAirportsRegions(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceOurAirportsTable(ctx, store.OurAirportsRegions, store.QueryDeleteAllRegions, columns, rows)
}

func (s *Store) ReplaceOurAirportsAirports(ctx context.Context, columns []string, rows [][]string) error {
	return s.replaceOurAirportsTable(ctx, store.OurAirportsAirports, store.QueryDeleteAllAirports, columns, rows)
}

func (s *Store) replaceOurAirportsTable(
	ctx context.Context,
	dataset store.OurAirportsDataset,
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

func hasOurAirportsDataQuery(dataset store.OurAirportsDataset) (string, error) {
	switch dataset {
	case store.OurAirportsCountries:
		return store.QueryHasCountriesData, nil
	case store.OurAirportsRegions:
		return store.QueryHasRegionsData, nil
	case store.OurAirportsAirports:
		return store.QueryHasAirportsData, nil
	default:
		return "", fmt.Errorf("%w: %q", store.ErrInvalidOurAirportsDataset, dataset)
	}
}
