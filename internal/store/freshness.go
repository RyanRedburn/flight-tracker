package store

import (
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

// FreshnessKind selects how a dataset's covered period is derived.
type FreshnessKind string

const (
	FreshnessKindMonth    FreshnessKind = model.FreshnessPeriodTypeMonth
	FreshnessKindSnapshot FreshnessKind = model.FreshnessPeriodTypeSnapshot
)

// FreshnessInput is the raw watermark for one dataset, read from Postgres.
// JobID empty means no completed job. HasMonth is set only when a covered
// month was found. For snapshots, RowCount is the primary table size and
// HasRows is true when that table or a sibling loaded by the same replace
// (airport_weather_stations) has rows.
type FreshnessInput struct {
	Kind     FreshnessKind
	JobID    string
	EndedAt  *time.Time
	Year     int
	Month    int
	HasMonth bool
	RowCount int64
	HasRows  bool
}

// AssembleDataFreshness builds the stable dataset list from raw watermarks.
// Missing inputs are reported as never ingested (null timestamps and period).
func AssembleDataFreshness(byID map[string]FreshnessInput) model.DataFreshness {
	ids := freshnessDatasetIDs()
	datasets := make([]model.DatasetFreshness, 0, len(ids))

	for _, id := range ids {
		in := byID[id]
		in.Kind = freshnessKindOrDefault(id, in.Kind)
		datasets = append(datasets, datasetFromFreshness(id, in))
	}

	return model.DataFreshness{Datasets: datasets}
}

func freshnessDatasetIDs() []string {
	return []string{
		model.DatasetIDFlightPerformance,
		model.DatasetIDWeatherObservations,
		model.DatasetIDWeatherStations,
		model.DatasetIDCountries,
		model.DatasetIDRegions,
		model.DatasetIDAirports,
	}
}

func freshnessKindOrDefault(id string, kind FreshnessKind) FreshnessKind {
	if kind != "" {
		return kind
	}

	switch id {
	case model.DatasetIDFlightPerformance, model.DatasetIDWeatherObservations:
		return FreshnessKindMonth
	case model.DatasetIDWeatherStations, model.DatasetIDCountries, model.DatasetIDRegions, model.DatasetIDAirports:
		return FreshnessKindSnapshot
	default:
		return ""
	}
}

func datasetFromFreshness(id string, in FreshnessInput) model.DatasetFreshness {
	ds := model.DatasetFreshness{ID: id}

	if in.JobID != "" {
		jobID := in.JobID
		ds.LastSuccessfulJobID = &jobID
		ds.LastSuccessfulIngestAt = cloneTime(in.EndedAt)
	}

	ds.LatestPeriod = freshnessPeriod(in)

	return ds
}

func freshnessPeriod(in FreshnessInput) *model.FreshnessPeriod {
	switch in.Kind {
	case FreshnessKindMonth:
		return monthFreshnessPeriod(in)
	case FreshnessKindSnapshot:
		return snapshotFreshnessPeriod(in)
	default:
		return nil
	}
}

func monthFreshnessPeriod(in FreshnessInput) *model.FreshnessPeriod {
	if !in.HasMonth || in.Year <= 0 || in.Month < 1 || in.Month > 12 {
		return nil
	}

	year := in.Year
	month := in.Month

	return &model.FreshnessPeriod{
		Type:  model.FreshnessPeriodTypeMonth,
		Year:  &year,
		Month: &month,
	}
}

func snapshotFreshnessPeriod(in FreshnessInput) *model.FreshnessPeriod {
	if in.JobID == "" && !in.HasRows {
		return nil
	}

	rowCount := in.RowCount
	period := &model.FreshnessPeriod{
		Type:     model.FreshnessPeriodTypeSnapshot,
		RowCount: &rowCount,
	}

	if in.JobID != "" {
		period.AsOf = cloneTime(in.EndedAt)
	}

	return period
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	utc := t.UTC()

	return &utc
}
