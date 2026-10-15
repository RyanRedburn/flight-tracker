package postgres

import (
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestBuildListJobsQuery(t *testing.T) {
	query, args := buildListJobsQuery(store.JobListFilter{Limit: 50})

	if strings.Contains(query, listJobsOpenStatusPlaceholder) || strings.Contains(query, listJobsLimitPlaceholder) {
		t.Fatalf("placeholders remain: %s", query)
	}

	if strings.Contains(query, "WHERE j.status") {
		t.Fatalf("default list should not filter status: %s", query)
	}

	if !strings.Contains(query, "LIMIT $1") || !strings.Contains(query, "ORDER BY j.created_at DESC") {
		t.Fatalf("query = %s", query)
	}

	if len(args) != 1 || args[0] != 50 {
		t.Fatalf("args = %#v", args)
	}
}

func TestBuildListJobsQueryOpenOnly(t *testing.T) {
	query, args := buildListJobsQuery(store.JobListFilter{Limit: 2, OpenJobsOnly: true})

	if strings.Contains(query, listJobsOpenStatusPlaceholder) || strings.Contains(query, listJobsLimitPlaceholder) {
		t.Fatalf("placeholders remain: %s", query)
	}

	if !strings.Contains(query, listJobsOpenStatusSQL) || !strings.Contains(query, "LIMIT $3") {
		t.Fatalf("query = %s", query)
	}

	if strings.Contains(query, "LIMIT $1") {
		t.Fatalf("open list should not keep LIMIT $1: %s", query)
	}

	if len(args) != 3 || args[0] != string(model.JobStatusPending) || args[1] != string(model.JobStatusRunning) || args[2] != 2 {
		t.Fatalf("args = %#v", args)
	}
}
