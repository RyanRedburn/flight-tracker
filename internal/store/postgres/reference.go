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

func (s *Store) CreateReferenceIngestJob(ctx context.Context, jobType model.JobType) (*model.Job, error) {
	if !isReferenceJobType(jobType) {
		return nil, fmt.Errorf("unsupported reference job type %q", jobType)
	}

	return s.insertPendingTypeJob(ctx, jobType, string(jobType))
}

func (s *Store) CreateRebuildRouteTravelWindowsJob(ctx context.Context) (*model.Job, error) {
	return s.insertPendingTypeJob(
		ctx,
		model.JobTypeRebuildRouteTravelWindows,
		store.TravelWindowRebuildJobLockKey,
	)
}

func (s *Store) CreateRebuildRouteWeatherStatsJob(ctx context.Context) (*model.Job, error) {
	return s.insertPendingTypeJob(
		ctx,
		model.JobTypeRebuildRouteWeatherStats,
		store.WeatherStatsRebuildJobLockKey,
	)
}

// insertPendingTypeJob inserts one pending jobs row for a parameterless job.
// lockKey is the advisory-lock namespace (hashtext); key 0 matches other type-only jobs.
func (s *Store) insertPendingTypeJob(ctx context.Context, jobType model.JobType, lockKey string) (*model.Job, error) {
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

	if _, err := tx.ExecContext(ctx, store.QueryAdvisoryXactLock, lockKey, 0); err != nil {
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

// insertPendingMonthJob inserts one pending month ingest job.
// The advisory lock key matches month replace: job type plus year*100+month.
// Each call commits its own transaction.
func (s *Store) insertPendingMonthJob(
	ctx context.Context,
	jobType model.JobType,
	year, month int,
	activeQuery string,
	insertDetail func(ctx context.Context, exec sqlExecContext, jobID string) error,
) (*model.Job, error) {
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

	if _, err := tx.ExecContext(ctx, store.QueryAdvisoryXactLock, string(jobType), year*100+month); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}

	rows, err := tx.QueryContext(ctx, activeQuery,
		string(model.JobStatusPending),
		string(model.JobStatusRunning),
	)
	if err != nil {
		return nil, err
	}

	active, scanErr := collectRequestedActiveMonths(rows, []model.YearMonth{{Year: year, Month: month}})
	if closeErr := rows.Close(); closeErr != nil && scanErr == nil {
		scanErr = closeErr
	}

	if scanErr != nil {
		return nil, scanErr
	}

	if len(active) > 0 {
		return nil, store.ErrActiveIngestConflict
	}

	if err := execCreateJob(ctx, tx, job); err != nil {
		return nil, err
	}

	if err := insertDetail(ctx, tx, job.ID); err != nil {
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

	return s.replaceTables(ctx, tableReplace{
		deleteQuery: deleteQuery,
		table:       table,
		columns:     columns,
		rows:        rows,
	})
}

func isReferenceJobType(jobType model.JobType) bool {
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
