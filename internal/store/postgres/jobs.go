package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) CreateFlightPerformanceIngestJob(ctx context.Context, year, month int) (*model.Job, error) {
	return s.insertPendingMonthJob(ctx, model.JobTypeImportFlightPerformance, year, month, store.QueryActiveFlightPerformanceIngestMonths,
		func(ctx context.Context, exec sqlExecContext, jobID string) error {
			if _, err := exec.ExecContext(ctx, store.QueryCreateFlightPerformanceIngestJob, jobID, year, month); err != nil {
				return fmt.Errorf("insert flight_performance_ingest_jobs: %w", err)
			}

			return nil
		},
	)
}

func (s *Store) GetFlightPerformanceIngestJob(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
	var detail model.FlightPerformanceIngestJob

	err := s.db.QueryRowContext(ctx, store.QueryGetFlightPerformanceIngestJob, jobID).Scan(
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

func (s *Store) CreateWeatherIngestJob(ctx context.Context, year, month int, stations []string) (*model.Job, error) {
	if len(stations) == 0 {
		return nil, errors.New("stations required")
	}

	stationsJSON, err := json.Marshal(stations)
	if err != nil {
		return nil, fmt.Errorf("marshal stations: %w", err)
	}

	return s.insertPendingMonthJob(ctx, model.JobTypeImportWeatherObservations, year, month, store.QueryActiveWeatherIngestMonths,
		func(ctx context.Context, exec sqlExecContext, jobID string) error {
			if _, err := exec.ExecContext(ctx, store.QueryCreateWeatherIngestJob, jobID, year, month, stationsJSON); err != nil {
				return fmt.Errorf("insert weather_ingest_jobs: %w", err)
			}

			return nil
		},
	)
}

func (s *Store) GetWeatherIngestJob(ctx context.Context, jobID string) (*model.WeatherIngestJob, error) {
	var detail model.WeatherIngestJob

	var stationsJSON []byte

	err := s.db.QueryRowContext(ctx, store.QueryGetWeatherIngestJob, jobID).Scan(
		&detail.JobID,
		&detail.Year,
		&detail.Month,
		&stationsJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("weather ingest job %q: %w", jobID, store.ErrNotFound)
	}

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(stationsJSON, &detail.Stations); err != nil {
		return nil, fmt.Errorf("unmarshal stations: %w", err)
	}

	return &detail, nil
}

func (s *Store) ClaimNextPendingJob(ctx context.Context, leaseUntil time.Time) (*model.Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, store.QueryClaimNextPendingJobSelect, string(model.JobStatusPending))

	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	var leaseExpires any
	if !leaseUntil.IsZero() {
		leaseExpires = leaseUntil.UTC()
	}

	res, err := tx.ExecContext(ctx, store.QueryClaimNextPendingJobUpdate,
		string(model.JobStatusRunning),
		now,
		now,
		leaseExpires,
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
	now := time.Now().UTC()

	res, err := s.db.ExecContext(ctx, store.QueryCompleteJob,
		string(model.JobStatusCompleted),
		nullJSON(result),
		nil,
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
	now := time.Now().UTC()

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

func (s *Store) HeartbeatJob(ctx context.Context, id string, leaseUntil time.Time) error {
	now := time.Now().UTC()

	res, err := s.db.ExecContext(ctx, store.QueryHeartbeatJob,
		leaseUntil.UTC(),
		now,
		id,
		string(model.JobStatusRunning),
	)
	if err != nil {
		return err
	}

	return expectOneRowAffected(res, store.ErrJobStatusConflict)
}

func (s *Store) ResetStaleRunningJobs(ctx context.Context, expiredBefore time.Time) (int64, error) {
	now := time.Now().UTC()

	res, err := s.db.ExecContext(ctx, store.QueryResetStaleRunningJobs,
		string(model.JobStatusPending),
		now,
		string(model.JobStatusRunning),
		expiredBefore.UTC(),
	)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (s *Store) ActiveFlightPerformanceIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	return s.activeIngestMonths(ctx, store.QueryActiveFlightPerformanceIngestMonths, months)
}

func (s *Store) ActiveWeatherIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	return s.activeIngestMonths(ctx, store.QueryActiveWeatherIngestMonths, months)
}

func (s *Store) activeIngestMonths(ctx context.Context, query string, months []model.YearMonth) ([]model.YearMonth, error) {
	if len(months) == 0 {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, query,
		string(model.JobStatusPending),
		string(model.JobStatusRunning),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectRequestedActiveMonths(rows, months)
}

func (s *Store) ActiveIngestJob(ctx context.Context, jobType model.JobType) (bool, error) {
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
	return s.monthsWithData(ctx, store.QueryMonthsWithFlightPerformanceData, months)
}

func (s *Store) MonthsWithWeatherData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	return s.monthsWithData(ctx, store.QueryMonthsWithWeatherData, months)
}

const monthsInPlaceholder = "/*months*/"

func (s *Store) monthsWithData(ctx context.Context, query string, months []model.YearMonth) ([]model.YearMonth, error) {
	if len(months) == 0 {
		return nil, nil
	}

	sqlQuery, args := expandMonthsIn(query, months)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[model.YearMonth]struct{}, len(months))

	for rows.Next() {
		var ym model.YearMonth
		if err := rows.Scan(&ym.Year, &ym.Month); err != nil {
			return nil, err
		}

		found[ym] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var withData []model.YearMonth

	for _, ym := range months {
		if _, ok := found[ym]; ok {
			withData = append(withData, ym)
		}
	}

	return withData, nil
}

func expandMonthsIn(query string, months []model.YearMonth) (string, []any) {
	var b strings.Builder

	args := make([]any, 0, len(months)*2)

	b.WriteByte('(')

	for i, ym := range months {
		if i > 0 {
			b.WriteString(", ")
		}

		fmt.Fprintf(&b, "($%d, $%d)", i*2+1, i*2+2)

		args = append(args, ym.Year, ym.Month)
	}

	b.WriteByte(')')

	return strings.Replace(query, monthsInPlaceholder, b.String(), 1), args
}

func collectRequestedActiveMonths(rows *sql.Rows, months []model.YearMonth) ([]model.YearMonth, error) {
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

func execCreateJob(ctx context.Context, exec sqlExecContext, job *model.Job) error {
	var errMsg sql.NullString
	if job.Error != "" {
		errMsg = sql.NullString{String: job.Error, Valid: true}
	}

	_, err := exec.ExecContext(ctx, store.QueryCreateJob,
		job.ID,
		job.Type,
		string(job.Status),
		nullJSON(job.Result),
		errMsg,
		job.CreatedAt.UTC(),
		job.UpdatedAt.UTC(),
		nullTime(job.StartedAt),
		nullTime(job.EndedAt),
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
