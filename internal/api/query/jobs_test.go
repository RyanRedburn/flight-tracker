package query

import (
	"net/http/httptest"
	"testing"
)

func TestParseJobsListDefaultLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/jobs", nil)

	limit, err := ParseJobsList(req)
	if err != nil {
		t.Fatalf("ParseJobsList() error = %v", err)
	}

	if limit != DefaultListLimit {
		t.Errorf("limit = %d, want %d", limit, DefaultListLimit)
	}
}

func TestParseJobsListRejectsLimitAboveMax(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/jobs?limit=501", nil)

	_, err := ParseJobsList(req)
	if err == nil {
		t.Fatal("ParseJobsList() expected error")
	}
}
