package store

import (
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const testFlightJobID = "job-fp"

func TestAssembleDataFreshnessEmpty(t *testing.T) {
	got := AssembleDataFreshness(nil)

	wantIDs := []string{
		model.DatasetIDFlightPerformance,
		model.DatasetIDWeatherObservations,
		model.DatasetIDWeatherStations,
		model.DatasetIDCountries,
		model.DatasetIDRegions,
		model.DatasetIDAirports,
	}

	if len(got.Datasets) != len(wantIDs) {
		t.Fatalf("len = %d, want %d", len(got.Datasets), len(wantIDs))
	}

	for i, ds := range got.Datasets {
		if ds.ID != wantIDs[i] {
			t.Errorf("datasets[%d].id = %q, want %q", i, ds.ID, wantIDs[i])
		}

		if ds.LastSuccessfulIngestAt != nil || ds.LatestPeriod != nil || ds.LastSuccessfulJobID != nil {
			t.Errorf("datasets[%d] = %+v, want empty", i, ds)
		}
	}
}

func TestAssembleDataFreshnessPartialBackfill(t *testing.T) {
	ended := time.Date(2026, 9, 18, 10, 22, 0, 123, time.FixedZone("EDT", -4*60*60))
	older := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	got := AssembleDataFreshness(map[string]FreshnessInput{
		model.DatasetIDFlightPerformance: {
			Kind:     FreshnessKindMonth,
			JobID:    testFlightJobID,
			EndedAt:  &ended,
			Year:     2026,
			Month:    6,
			HasMonth: true,
		},
		model.DatasetIDWeatherObservations: {
			Kind:     FreshnessKindMonth,
			Year:     2024,
			Month:    1,
			HasMonth: true,
		},
		model.DatasetIDWeatherStations: {
			Kind:     FreshnessKindSnapshot,
			RowCount: 0,
			HasRows:  true,
		},
		model.DatasetIDCountries: {
			Kind:     FreshnessKindSnapshot,
			JobID:    "job-countries",
			EndedAt:  &older,
			RowCount: 247,
			HasRows:  true,
		},
		model.DatasetIDRegions: {
			Kind:     FreshnessKindSnapshot,
			JobID:    "job-regions-empty",
			EndedAt:  &older,
			RowCount: 0,
			HasRows:  false,
		},
	})

	byID := indexFreshness(t, got)

	fp := byID[model.DatasetIDFlightPerformance]
	if fp.LastSuccessfulJobID == nil || *fp.LastSuccessfulJobID != testFlightJobID {
		t.Fatalf("flight job id = %v, want %s", fp.LastSuccessfulJobID, testFlightJobID)
	}

	if fp.LastSuccessfulIngestAt == nil || !fp.LastSuccessfulIngestAt.Equal(ended.UTC()) {
		t.Fatalf("flight ingest at = %v, want %s", fp.LastSuccessfulIngestAt, ended.UTC())
	}

	if fp.LatestPeriod == nil || fp.LatestPeriod.Type != model.FreshnessPeriodTypeMonth {
		t.Fatalf("flight period = %+v, want month", fp.LatestPeriod)
	}

	if fp.LatestPeriod.Year == nil || *fp.LatestPeriod.Year != 2026 || fp.LatestPeriod.Month == nil || *fp.LatestPeriod.Month != 6 {
		t.Fatalf("flight period = %+v, want 2026-06", fp.LatestPeriod)
	}

	if fp.LatestPeriod.AsOf != nil || fp.LatestPeriod.RowCount != nil {
		t.Fatalf("flight period includes snapshot fields: %+v", fp.LatestPeriod)
	}

	wx := byID[model.DatasetIDWeatherObservations]
	if wx.LastSuccessfulIngestAt != nil || wx.LastSuccessfulJobID != nil {
		t.Fatalf("weather ingest = %+v, want no job", wx)
	}

	if wx.LatestPeriod == nil || wx.LatestPeriod.Year == nil || *wx.LatestPeriod.Year != 2024 || wx.LatestPeriod.Month == nil || *wx.LatestPeriod.Month != 1 {
		t.Fatalf("weather period = %+v, want 2024-01", wx.LatestPeriod)
	}

	stations := byID[model.DatasetIDWeatherStations]
	if stations.LastSuccessfulIngestAt != nil || stations.LastSuccessfulJobID != nil {
		t.Fatalf("stations ingest = %+v, want no job", stations)
	}

	if stations.LatestPeriod == nil || stations.LatestPeriod.Type != model.FreshnessPeriodTypeSnapshot {
		t.Fatalf("stations period = %+v, want snapshot", stations.LatestPeriod)
	}

	if stations.LatestPeriod.AsOf != nil {
		t.Fatalf("stations as_of = %v, want nil", stations.LatestPeriod.AsOf)
	}

	if stations.LatestPeriod.RowCount == nil || *stations.LatestPeriod.RowCount != 0 {
		t.Fatalf("stations row_count = %v, want 0", stations.LatestPeriod.RowCount)
	}

	countries := byID[model.DatasetIDCountries]
	if countries.LatestPeriod == nil || countries.LatestPeriod.RowCount == nil || *countries.LatestPeriod.RowCount != 247 {
		t.Fatalf("countries period = %+v, want row_count 247", countries.LatestPeriod)
	}

	if countries.LatestPeriod.AsOf == nil || !countries.LatestPeriod.AsOf.Equal(older) {
		t.Fatalf("countries as_of = %v, want %s", countries.LatestPeriod.AsOf, older)
	}

	regions := byID[model.DatasetIDRegions]
	if regions.LastSuccessfulJobID == nil || *regions.LastSuccessfulJobID != "job-regions-empty" {
		t.Fatalf("regions job = %v", regions.LastSuccessfulJobID)
	}

	if regions.LatestPeriod == nil || regions.LatestPeriod.RowCount == nil || *regions.LatestPeriod.RowCount != 0 {
		t.Fatalf("regions period = %+v, want row_count 0", regions.LatestPeriod)
	}

	airports := byID[model.DatasetIDAirports]
	if airports.LatestPeriod != nil || airports.LastSuccessfulJobID != nil {
		t.Fatalf("airports = %+v, want empty", airports)
	}
}

func TestAssembleDataFreshnessIgnoresInvalidMonth(t *testing.T) {
	got := AssembleDataFreshness(map[string]FreshnessInput{
		model.DatasetIDFlightPerformance: {
			Kind:     FreshnessKindMonth,
			JobID:    testFlightJobID,
			HasMonth: true,
			Year:     2026,
			Month:    13,
		},
	})

	fp := indexFreshness(t, got)[model.DatasetIDFlightPerformance]
	if fp.LastSuccessfulJobID == nil {
		t.Fatal("expected job id")
	}

	if fp.LatestPeriod != nil {
		t.Fatalf("period = %+v, want nil", fp.LatestPeriod)
	}
}

func indexFreshness(t *testing.T, got model.DataFreshness) map[string]model.DatasetFreshness {
	t.Helper()

	out := make(map[string]model.DatasetFreshness, len(got.Datasets))
	for _, ds := range got.Datasets {
		out[ds.ID] = ds
	}

	return out
}
