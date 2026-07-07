package query

import (
	"net/http/httptest"
	"testing"
)

func TestParseFlightsListDefaultLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/flights", nil)

	filter, err := ParseFlightsList(req)
	if err != nil {
		t.Fatalf("ParseFlightsList() error = %v", err)
	}

	if filter.Limit != DefaultListLimit {
		t.Errorf("Limit = %d, want %d", filter.Limit, DefaultListLimit)
	}
}

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

func TestParseFlightsListRejectsLimitAboveMax(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/flights?limit=501", nil)

	_, err := ParseFlightsList(req)
	if err == nil {
		t.Fatal("ParseFlightsList() expected error")
	}
}

func TestParseJobsListRejectsLimitAboveMax(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/jobs?limit=501", nil)

	_, err := ParseJobsList(req)
	if err == nil {
		t.Fatal("ParseJobsList() expected error")
	}
}
