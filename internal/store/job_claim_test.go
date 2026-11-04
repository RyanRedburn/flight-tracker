package store

import (
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

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
