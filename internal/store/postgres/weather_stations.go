package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) ReplaceWeatherStations(
	ctx context.Context,
	stationColumns []string,
	stationRows [][]string,
	mappingColumns []string,
	mappingRows [][]string,
) error {
	return s.replaceTables(ctx,
		tableReplace{
			deleteQuery: store.QueryDeleteAllAirportWeatherStations,
			table:       "airport_weather_stations",
			columns:     mappingColumns,
			rows:        mappingRows,
		},
		tableReplace{
			deleteQuery: store.QueryDeleteAllWeatherStations,
			table:       "weather_stations",
			columns:     stationColumns,
			rows:        stationRows,
		},
	)
}

func (s *Store) HasWeatherStationsData(ctx context.Context) (bool, error) {
	var exists int

	err := s.db.QueryRowContext(ctx, store.QueryHasWeatherStationsData).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Store) ListAirportWeatherStations(ctx context.Context) ([]store.AirportWeatherStation, error) {
	rows, err := s.db.QueryContext(ctx, store.QueryListAirportWeatherStations)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]store.AirportWeatherStation, 0)

	for rows.Next() {
		var (
			row       store.AirportWeatherStation
			iemSID    sql.NullString
			tzname    sql.NullString
			updatedAt sql.NullTime
		)

		if err := rows.Scan(
			&row.AirportCode,
			&iemSID,
			&tzname,
			&row.Matched,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		row.IEMSID = iemSID.String
		row.TzName = tzname.String

		if updatedAt.Valid {
			row.UpdatedAt = updatedAt.Time
		}

		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
