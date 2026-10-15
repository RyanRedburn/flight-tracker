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

	if !strings.Contains(query, "created_at ASC") {
		t.Fatalf("query = %s", query)
	}

	wantIn := "IN ('" + string(model.JobTypeRebuildRouteTravelWindows) + "', '" + string(model.JobTypeRebuildRouteWeatherStats) + "')"
	if strings.Count(query, wantIn) != 2 {
		t.Fatalf("rebuild type list count = %d, want 2 (rank and running exclusion): %s", strings.Count(query, wantIn), query)
	}

	if !strings.Contains(query, "running.type = candidate.type") || !strings.Contains(query, "running.status = $2") {
		t.Fatalf("query = %s, want a same-type running-job exclusion", query)
	}

	if !strings.Contains(query, "FOR UPDATE OF candidate SKIP LOCKED") {
		t.Fatalf("query = %s", query)
	}

	if strings.Contains(query, "'"+string(model.JobTypeImportFlightPerformance)+"'") {
		t.Fatalf("ingest type should stay at rank 0: %s", query)
	}
}

func TestNextPendingClaimLeavesRunningRebuild(t *testing.T) {
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	const (
		idTravelRunning  = "travel-running"
		idTravelPending  = "travel-pending"
		idWeatherPending = "weather-pending"
		idFlightsPending = "flights-pending"
	)

	travel := model.JobTypeRebuildRouteTravelWindows
	weather := model.JobTypeRebuildRouteWeatherStats
	flights := model.JobTypeImportFlightPerformance

	tests := []struct {
		name string
		jobs []model.Job
		want string
	}{
		{
			name: "same type running",
			jobs: []model.Job{
				{ID: idTravelRunning, Type: travel, Status: model.JobStatusRunning, CreatedAt: base},
				{ID: idTravelPending, Type: travel, Status: model.JobStatusPending, CreatedAt: base.Add(time.Minute)},
			},
			want: "",
		},
		{
			name: "other rebuild type still claimable",
			jobs: []model.Job{
				{ID: idTravelRunning, Type: travel, Status: model.JobStatusRunning, CreatedAt: base},
				{ID: idTravelPending, Type: travel, Status: model.JobStatusPending, CreatedAt: base.Add(time.Minute)},
				{ID: idWeatherPending, Type: weather, Status: model.JobStatusPending, CreatedAt: base.Add(2 * time.Minute)},
			},
			want: idWeatherPending,
		},
		{
			name: "ingest before a free rebuild",
			jobs: []model.Job{
				{ID: idWeatherPending, Type: weather, Status: model.JobStatusPending, CreatedAt: base},
				{ID: idFlightsPending, Type: flights, Status: model.JobStatusPending, CreatedAt: base.Add(time.Hour)},
				{ID: idTravelRunning, Type: travel, Status: model.JobStatusRunning, CreatedAt: base},
			},
			want: idFlightsPending,
		},
		{
			name: "oldest free rebuild",
			jobs: []model.Job{
				{ID: "weather-new", Type: weather, Status: model.JobStatusPending, CreatedAt: base.Add(time.Hour)},
				{ID: "travel-old", Type: travel, Status: model.JobStatusPending, CreatedAt: base},
			},
			want: "travel-old",
		},
		{
			name: "running ingest does not block",
			jobs: []model.Job{
				{ID: "flights-running", Type: flights, Status: model.JobStatusRunning, CreatedAt: base},
				{ID: idFlightsPending, Type: flights, Status: model.JobStatusPending, CreatedAt: base.Add(time.Minute)},
				{ID: idTravelPending, Type: travel, Status: model.JobStatusPending, CreatedAt: base},
			},
			want: idFlightsPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nextPendingClaim(tt.jobs)
			if tt.want == "" {
				if ok {
					t.Fatalf("claimed %s, want none", got.ID)
				}

				return
			}

			if !ok || got.ID != tt.want {
				t.Fatalf("claimed %q, want %s", got.ID, tt.want)
			}
		})
	}
}

func nextPendingClaim(jobs []model.Job) (model.Job, bool) {
	running := make(map[model.JobType]struct{})

	for _, job := range jobs {
		if job.Status == model.JobStatusRunning && jobClaimRank(job.Type) > 0 {
			running[job.Type] = struct{}{}
		}
	}

	var pending []model.Job

	for _, job := range jobs {
		if job.Status != model.JobStatusPending {
			continue
		}

		if _, blocked := running[job.Type]; blocked && jobClaimRank(job.Type) > 0 {
			continue
		}

		pending = append(pending, job)
	}

	if len(pending) == 0 {
		return model.Job{}, false
	}

	sort.Slice(pending, func(i, j int) bool {
		return jobClaimLess(pending[i], pending[j])
	})

	return pending[0], true
}

func jobIDs(jobs []model.Job) []string {
	ids := make([]string, len(jobs))
	for i, job := range jobs {
		ids[i] = job.ID
	}

	return ids
}
