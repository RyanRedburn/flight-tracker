package model

import "time"

const (
	DatasetIDFlightPerformance   = "flight_performance"
	DatasetIDWeatherObservations = "weather_observations"
	DatasetIDWeatherStations     = "weather_stations"
	DatasetIDCountries           = "countries"
	DatasetIDRegions             = "regions"
	DatasetIDAirports            = "airports"

	FreshnessPeriodTypeMonth    = "month"
	FreshnessPeriodTypeSnapshot = "snapshot"
)

// DataFreshness is the per-dataset ingest watermark and covered period.
type DataFreshness struct {
	Datasets []DatasetFreshness
}

// DatasetFreshness is one current dataset. Nil fields mean that signal is absent.
type DatasetFreshness struct {
	ID                     string
	LastSuccessfulIngestAt *time.Time
	LatestPeriod           *FreshnessPeriod
	LastSuccessfulJobID    *string
}

// FreshnessPeriod is either a covered month or a full-table snapshot.
type FreshnessPeriod struct {
	Type     string
	Year     *int
	Month    *int
	AsOf     *time.Time
	RowCount *int64
}
