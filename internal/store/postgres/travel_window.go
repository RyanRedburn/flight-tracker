package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) RouteTravelWindows(ctx context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
	var (
		windowStart time.Time
		windowEnd   time.Time
		flights     int
	)

	err := s.db.QueryRowContext(
		ctx,
		store.QueryRouteTravelWindowScope,
		filter.Origin,
		filter.Dest,
		filter.Carrier,
	).Scan(&windowStart, &windowEnd, &flights)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("route travel windows %s-%s: %w", filter.Origin, filter.Dest, store.ErrNotFound)
	}

	if err != nil {
		return nil, err
	}

	if flights < 1 {
		return nil, fmt.Errorf("route travel windows %s-%s: %w", filter.Origin, filter.Dest, store.ErrNotFound)
	}

	rows, err := s.db.QueryContext(
		ctx,
		store.QueryRouteTravelWindowBuckets,
		filter.Origin,
		filter.Dest,
		filter.Carrier,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]store.TravelWindowBucketCount, 0)

	for rows.Next() {
		var count store.TravelWindowBucketCount
		if err := rows.Scan(&count.Grain, &count.Bucket, &count.OnTimeCount, &count.Flights); err != nil {
			return nil, err
		}

		counts = append(counts, count)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return store.AssembleRouteTravelWindows(
		filter.Origin,
		filter.Dest,
		filter.Carrier,
		windowStart.Format("2006-01-02"),
		windowEnd.Format("2006-01-02"),
		flights,
		counts,
	), nil
}

// RebuildRouteTravelWindows replaces route and route+carrier travel-window
// rollups from flight_performance. Call after a successful flight-performance
// month load. An advisory xact lock makes concurrent replicas serialize;
// the rebuild is a full truncate+insert, so repeating it is idempotent.
func (s *Store) RebuildRouteTravelWindows(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, store.QueryAdvisoryXactLock, store.TravelWindowRebuildLockKey, 0); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}

	if _, err := tx.ExecContext(ctx, store.QueryTruncateRouteTravelWindows); err != nil {
		return fmt.Errorf("truncate travel windows: %w", err)
	}

	if _, err := tx.ExecContext(ctx, store.QueryInsertRouteTravelWindows); err != nil {
		return fmt.Errorf("insert travel windows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
