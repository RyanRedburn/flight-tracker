package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const weatherStatsExtraPlaceholder = "/*wxextra*/"

func (s *Store) RouteWeatherStats(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteWeatherStats, error) {
	// Identity is flight_performance, not the rollup. A known route with no
	// bucket rows (date filter, or weather not yet rebuilt) is an empty 200.
	if err := s.ensureRouteIdentity(ctx, filter.Origin, filter.Dest, filter.Carrier); err != nil {
		return nil, err
	}

	query, args := buildRouteWeatherStatsQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]store.WeatherCategoryCount, 0)

	for rows.Next() {
		var count store.WeatherCategoryCount
		if err := rows.Scan(&count.Side, &count.Category, &count.OnTimeCount, &count.Flights); err != nil {
			return nil, err
		}

		counts = append(counts, count)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return store.AssembleRouteWeatherStats(filter, counts), nil
}

// RebuildRouteWeatherStats replaces per-side route weather rollups from
// flight_performance joined to the nearest classified observation. Call after
// a successful flight-performance load, weather-observation load, or weather
// station mapping replace. An advisory xact lock serializes replicas; the
// rebuild is a full truncate+insert, so repeating it is idempotent.
func (s *Store) RebuildRouteWeatherStats(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, store.QueryAdvisoryXactLock, store.WeatherStatsRebuildLockKey, 0); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}

	if _, err := tx.ExecContext(ctx, store.QueryTruncateRouteWeatherStats); err != nil {
		return fmt.Errorf("truncate route weather stats: %w", err)
	}

	if _, err := tx.ExecContext(ctx, store.QueryInsertRouteWeatherStats); err != nil {
		return fmt.Errorf("insert route weather stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func buildRouteWeatherStatsQuery(filter store.RouteStatsFilter) (string, []any) {
	args := []any{filter.Origin, filter.Dest, filter.StartDate, filter.EndDate}
	n := 5

	var extra strings.Builder

	if filter.Carrier != "" {
		fmt.Fprintf(&extra, " AND carrier = $%d", n)

		args = append(args, filter.Carrier)
		n++
	}

	if filter.FlightNumber != "" {
		fmt.Fprintf(&extra, " AND flight_number::text = $%d", n)

		args = append(args, filter.FlightNumber)
		n++
	}

	if len(filter.DaysOfWeek) > 0 {
		fmt.Fprintf(&extra, " AND day_of_week = ANY($%d)", n)

		args = append(args, filter.DaysOfWeek)
	}

	query := strings.Replace(store.QueryRouteWeatherStats, weatherStatsExtraPlaceholder, extra.String(), 1)

	return query, args
}
