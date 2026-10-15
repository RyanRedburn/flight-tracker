package store

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestJobClaimRank(t *testing.T) {
	ingest := []model.JobType{
		model.JobTypeImportFlightPerformance,
		model.JobTypeImportWeatherObservations,
		model.JobTypeImportCountries,
		model.JobTypeImportRegions,
		model.JobTypeImportAirports,
		model.JobTypeImportAirportMCT,
		model.JobTypeImportWeatherStations,
	}

	for _, jobType := range ingest {
		if got := jobClaimRank(jobType); got != 0 {
			t.Errorf("jobClaimRank(%q) = %d, want 0", jobType, got)
		}
	}

	for _, jobType := range rebuildClaimJobTypes {
		if got := jobClaimRank(jobType); got != 1 {
			t.Errorf("jobClaimRank(%q) = %d, want 1", jobType, got)
		}
	}
}

func TestJobClaimOrder(t *testing.T) {
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	jobs := []model.Job{
		{ID: "rebuild-old", Type: model.JobTypeRebuildRouteTravelWindows, CreatedAt: base},
		{ID: "wx-new", Type: model.JobTypeImportWeatherObservations, CreatedAt: base.Add(2 * time.Hour)},
		{ID: "fp-mid", Type: model.JobTypeImportFlightPerformance, CreatedAt: base.Add(time.Hour)},
		{ID: "rebuild-new", Type: model.JobTypeRebuildRouteWeatherStats, CreatedAt: base.Add(3 * time.Hour)},
		{ID: "import-airports", Type: model.JobTypeImportAirports, CreatedAt: base.Add(3 * time.Hour)},
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobClaimLess(jobs[i], jobs[j])
	})

	want := []string{"fp-mid", "wx-new", "import-airports", "rebuild-old", "rebuild-new"}
	for i, id := range want {
		if jobs[i].ID != id {
			t.Fatalf("order = %v, want %v", jobIDs(jobs), want)
		}
	}
}

func TestClaimNextPendingJobQuery(t *testing.T) {
	query := ClaimNextPendingJobQuery()

	if strings.Contains(query, rebuildTypesPlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if !strings.Contains(query, "FOR UPDATE SKIP LOCKED") {
		t.Fatalf("query = %s", query)
	}

	if !strings.Contains(query, "created_at ASC") {
		t.Fatalf("query = %s", query)
	}

	wantIn := "IN ('" + string(model.JobTypeRebuildRouteTravelWindows) + "', '" + string(model.JobTypeRebuildRouteWeatherStats) + "')"
	if !strings.Contains(query, wantIn) {
		t.Fatalf("query = %s, want %s", query, wantIn)
	}

	if strings.Contains(query, "'"+string(model.JobTypeImportFlightPerformance)+"'") {
		t.Fatalf("ingest type should stay at rank 0: %s", query)
	}
}

func jobIDs(jobs []model.Job) []string {
	ids := make([]string, len(jobs))
	for i, job := range jobs {
		ids[i] = job.ID
	}

	return ids
}
