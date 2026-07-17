package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/uuid"
)

func (s *Store) CreateFlightPerformanceIngestJob(ctx context.Context, year, month int) (*model.Job, error) {
	now := time.Now().UTC()
	job := &model.Job{
		ID:        uuid.NewString(),
		Type:      model.JobTypeImportFlightPerformance,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := execCreateJob(ctx, tx, job); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, store.QueryCreateFlightPerformanceIngestJob, job.ID, year, month); err != nil {
		return nil, fmt.Errorf("insert flight_performance_ingest_jobs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return job, nil
}

func (s *Store) GetFlightPerformanceIngestJob(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
	var detail model.FlightPerformanceIngestJob

	err := s.db.QueryRowxContext(ctx, store.QueryGetFlightPerformanceIngestJob, jobID).Scan(
		&detail.JobID,
		&detail.Year,
		&detail.Month,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("flight performance ingest job %q: %w", jobID, store.ErrNotFound)
	}

	if err != nil {
		return nil, err
	}

	return &detail, nil
}

func (s *Store) ClaimNextPendingJob(ctx context.Context) (*model.Job, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowxContext(ctx, store.QueryClaimNextPendingJobSelect, string(model.JobStatusPending))

	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	nowStr := now.UTC().Format(time.RFC3339)

	res, err := tx.ExecContext(ctx, store.QueryClaimNextPendingJobUpdate,
		string(model.JobStatusRunning),
		nowStr,
		nowStr,
		job.ID,
		string(model.JobStatusPending),
	)
	if err != nil {
		return nil, fmt.Errorf("claim job: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if affected != 1 {
		return nil, store.ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	job.Status = model.JobStatusRunning
	job.StartedAt = &now
	job.UpdatedAt = now

	return job, nil
}

func (s *Store) CompleteJob(ctx context.Context, id string, result json.RawMessage) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var resultVal sql.NullString
	if len(result) > 0 {
		resultVal = sql.NullString{String: string(result), Valid: true}
	}

	res, err := s.db.ExecContext(ctx, store.QueryCompleteJob,
		string(model.JobStatusCompleted),
		resultVal,
		sql.NullString{},
		now,
		now,
		id,
		string(model.JobStatusRunning),
	)
	if err != nil {
		return err
	}

	return expectOneRowAffected(res, store.ErrJobStatusConflict)
}

func (s *Store) FailJob(ctx context.Context, id, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx, store.QueryFailJob,
		string(model.JobStatusFailed),
		errMsg,
		now,
		now,
		id,
		string(model.JobStatusRunning),
	)
	if err != nil {
		return err
	}

	return expectOneRowAffected(res, store.ErrJobStatusConflict)
}

func (s *Store) ResetStaleRunningJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	cutoff := olderThan.UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx, store.QueryResetStaleRunningJobs,
		string(model.JobStatusPending),
		now,
		string(model.JobStatusRunning),
		cutoff,
	)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (s *Store) ActiveFlightPerformanceIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if len(months) == 0 {
		return nil, nil
	}

	rows, err := s.db.QueryxContext(ctx, store.QueryActiveFlightPerformanceIngestMonths,
		string(model.JobStatusPending),
		string(model.JobStatusRunning),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activeSet := make(map[model.YearMonth]struct{}, len(months))

	requested := make(map[model.YearMonth]struct{}, len(months))
	for _, ym := range months {
		requested[ym] = struct{}{}
	}

	var active []model.YearMonth

	for rows.Next() {
		var ym model.YearMonth
		if err := rows.Scan(&ym.Year, &ym.Month); err != nil {
			return nil, err
		}

		if _, ok := requested[ym]; !ok {
			continue
		}

		if _, seen := activeSet[ym]; seen {
			continue
		}

		activeSet[ym] = struct{}{}
		active = append(active, ym)
	}

	return active, rows.Err()
}

func (s *Store) ActiveIngestJob(ctx context.Context, jobType string) (bool, error) {
	var exists int

	err := s.db.QueryRowContext(ctx, store.QueryActiveIngestJob,
		jobType,
		string(model.JobStatusPending),
		string(model.JobStatusRunning),
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Store) MonthsWithFlightPerformanceData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	var withData []model.YearMonth

	for _, ym := range months {
		var exists int

		err := s.db.QueryRowContext(ctx, store.QueryMonthsWithFlightPerformanceData,
			strconv.Itoa(ym.Year),
			strconv.Itoa(ym.Month),
		).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}

		if err != nil {
			return nil, err
		}

		withData = append(withData, ym)
	}

	return withData, nil
}

func (s *Store) ReplaceFlightPerformanceByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return errors.New("columns required")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, store.QueryDeleteFlightPerformanceByMonth,
		strconv.Itoa(year),
		strconv.Itoa(month),
	); err != nil {
		return fmt.Errorf("delete month rows: %w", err)
	}

	if len(rows) == 0 {
		return tx.Commit()
	}

	if err := replaceTableRows(ctx, tx, "flight_performance", columns, rows, false); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func execCreateJob(ctx context.Context, exec sqlExecContext, job *model.Job) error {
	var result sql.NullString
	if len(job.Result) > 0 {
		result = sql.NullString{String: string(job.Result), Valid: true}
	}

	var errMsg sql.NullString
	if job.Error != "" {
		errMsg = sql.NullString{String: job.Error, Valid: true}
	}

	_, err := exec.ExecContext(ctx, store.QueryCreateJob,
		job.ID,
		job.Type,
		string(job.Status),
		result,
		errMsg,
		job.CreatedAt.UTC().Format(time.RFC3339),
		job.UpdatedAt.UTC().Format(time.RFC3339),
		nullTimeString(job.StartedAt),
		nullTimeString(job.EndedAt),
	)

	return err
}

type sqlExecContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func expectOneRowAffected(res sql.Result, conflictErr error) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if affected != 1 {
		return conflictErr
	}

	return nil
}

func quoteSQLiteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func nullTimeString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}

func parseOptionalTime(raw sql.NullString) (*time.Time, error) {
	if !raw.Valid {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw.String)
	if err != nil {
		return nil, fmt.Errorf("parse time %q: %w", raw.String, err)
	}

	utc := parsed.UTC()

	return &utc, nil
}
