package postgres

import (
	"context"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) DistinctFlightAirportCodes(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryxContext(ctx, store.QueryDistinctFlightAirportCodes)
	if err != nil {
		return nil, fmt.Errorf("query distinct flight airports: %w", err)
	}
	defer rows.Close()

	var codes []string

	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}

		codes = append(codes, code)
	}

	return codes, rows.Err()
}
