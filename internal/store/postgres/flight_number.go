package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) FlightNumberRoutes(ctx context.Context, filter store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
	rows, err := s.db.QueryContext(ctx, store.QueryFlightNumberRoutes,
		filter.Carrier,
		filter.FlightNumber,
		store.OutlookLookbackDays,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	out := make([]model.FlightNumberRoute, 0)

	for rows.Next() {
		var (
			route      model.FlightNumberRoute
			medianMins sql.NullFloat64
		)

		if err := rows.Scan(&route.Origin, &route.Dest, &route.Flights, &medianMins); err != nil {
			return nil, err
		}

		if medianMins.Valid {
			route.UsualDepTime = store.MinutesToHHMM(int(medianMins.Float64))
		}

		out = append(out, route)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("flight number routes %s %s: %w", filter.Carrier, filter.FlightNumber, store.ErrNotFound)
	}

	return out, nil
}
