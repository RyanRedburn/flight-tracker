package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type sqlQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type sqlQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// lockIngestJob takes a transaction-scoped advisory lock for a job type and month.
// Use year=0, month=0 for type-only jobs (reference / weather-stations).
//
// Check-then-insert in a transaction without this lock is not enough: under
// READ COMMITTED, two sessions can both observe no active row and both insert.
func lockIngestJob(ctx context.Context, exec sqlExecContext, jobType string, year, month int) error {
	if _, err := exec.ExecContext(ctx, store.QueryAdvisoryXactLock, jobType, year*100+month); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}

	return nil
}

func listActiveRequestedMonths(ctx context.Context, q sqlQueryer, query string, months []model.YearMonth) ([]model.YearMonth, error) {
	rows, err := q.QueryContext(ctx, query,
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

func activeIngestJob(ctx context.Context, q sqlQueryRower, jobType string) (bool, error) {
	var exists int

	err := q.QueryRowContext(ctx, store.QueryActiveIngestJob,
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
