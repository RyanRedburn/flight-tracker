package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type jobWatermark struct {
	id      sql.NullString
	endedAt sql.NullTime
}

func (s *Store) DataFreshness(ctx context.Context) (model.DataFreshness, error) {
	var (
		flightJob                 jobWatermark
		flightYear, flightMonth   sql.NullInt64
		weatherJob                jobWatermark
		weatherYear, weatherMonth sql.NullInt64
		stationsJob               jobWatermark
		stationsCount             int64
		mappingsCount             int64
		countriesJob              jobWatermark
		countriesCount            int64
		regionsJob                jobWatermark
		regionsCount              int64
		airportsJob               jobWatermark
		airportsCount             int64
	)

	err := s.db.QueryRowContext(ctx, store.QueryDataFreshness,
		string(model.JobStatusCompleted),
		model.JobTypeImportFlightPerformance,
		model.JobTypeImportWeatherObservations,
		model.JobTypeImportWeatherStations,
		model.JobTypeImportCountries,
		model.JobTypeImportRegions,
		model.JobTypeImportAirports,
	).Scan(
		&flightJob.id,
		&flightJob.endedAt,
		&flightYear,
		&flightMonth,
		&weatherJob.id,
		&weatherJob.endedAt,
		&weatherYear,
		&weatherMonth,
		&stationsJob.id,
		&stationsJob.endedAt,
		&stationsCount,
		&mappingsCount,
		&countriesJob.id,
		&countriesJob.endedAt,
		&countriesCount,
		&regionsJob.id,
		&regionsJob.endedAt,
		&regionsCount,
		&airportsJob.id,
		&airportsJob.endedAt,
		&airportsCount,
	)
	if err != nil {
		return model.DataFreshness{}, fmt.Errorf("query data freshness: %w", err)
	}

	byID := map[string]store.FreshnessInput{
		model.DatasetIDFlightPerformance:   flightJob.toInput(store.FreshnessKindMonth, flightYear, flightMonth, 0, false),
		model.DatasetIDWeatherObservations: weatherJob.toInput(store.FreshnessKindMonth, weatherYear, weatherMonth, 0, false),
		model.DatasetIDWeatherStations: stationsJob.toInput(
			store.FreshnessKindSnapshot, sql.NullInt64{}, sql.NullInt64{}, stationsCount, stationsCount > 0 || mappingsCount > 0,
		),
		model.DatasetIDCountries: countriesJob.toInput(store.FreshnessKindSnapshot, sql.NullInt64{}, sql.NullInt64{}, countriesCount, countriesCount > 0),
		model.DatasetIDRegions:   regionsJob.toInput(store.FreshnessKindSnapshot, sql.NullInt64{}, sql.NullInt64{}, regionsCount, regionsCount > 0),
		model.DatasetIDAirports:  airportsJob.toInput(store.FreshnessKindSnapshot, sql.NullInt64{}, sql.NullInt64{}, airportsCount, airportsCount > 0),
	}

	return store.AssembleDataFreshness(byID), nil
}

func (w jobWatermark) toInput(kind store.FreshnessKind, year, month sql.NullInt64, rowCount int64, hasRows bool) store.FreshnessInput {
	in := store.FreshnessInput{
		Kind:     kind,
		RowCount: rowCount,
		HasRows:  hasRows,
	}

	if w.id.Valid {
		in.JobID = w.id.String
	}

	if w.endedAt.Valid {
		ended := w.endedAt.Time
		in.EndedAt = &ended
	}

	if year.Valid && month.Valid {
		in.Year = int(year.Int64)
		in.Month = int(month.Int64)
		in.HasMonth = true
	}

	return in
}
