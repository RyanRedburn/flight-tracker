package query

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseCarrierStats(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats?carrier=ua&state=il&start_date=2025-01-01&end_date=2025-01-31", nil)

	filter, err := ParseCarrierStats(req)
	if err != nil {
		t.Fatalf("ParseCarrierStats() error = %v", err)
	}

	if filter.Carrier != "UA" {
		t.Errorf("carrier = %q, want UA", filter.Carrier)
	}

	if filter.State != "IL" {
		t.Errorf("state = %q, want IL", filter.State)
	}

	if filter.StartDate != "2025-01-01" || filter.EndDate != "2025-01-31" {
		t.Errorf("dates = %s..%s", filter.StartDate, filter.EndDate)
	}
}

func TestParseCarrierStatsDefaultDates(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats?carrier=UA", nil)

	filter, err := ParseCarrierStats(req)
	if err != nil {
		t.Fatalf("ParseCarrierStats() error = %v", err)
	}

	if filter.StartDate != "" || filter.EndDate != "" {
		t.Errorf("expected empty dates, got %s..%s", filter.StartDate, filter.EndDate)
	}
}

func TestParseCarrierStatsValidation(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"missing carrier", "/api/v1/carriers/stats"},
		{"carrier too short", "/api/v1/carriers/stats?carrier=U"},
		{"start without end", "/api/v1/carriers/stats?carrier=UA&start_date=2025-01-01"},
		{"end without start", "/api/v1/carriers/stats?carrier=UA&end_date=2025-01-31"},
		{"end before start", "/api/v1/carriers/stats?carrier=UA&start_date=2025-02-01&end_date=2025-01-01"},
		{"span too long", "/api/v1/carriers/stats?carrier=UA&start_date=2024-01-01&end_date=2025-12-31"},
		{"state too long", "/api/v1/carriers/stats?carrier=UA&state=ILL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if _, err := ParseCarrierStats(req); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseCarrierStatsSpanOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats?carrier=UA&start_date=2025-01-01&end_date=2025-12-31", nil)

	filter, err := ParseCarrierStats(req)
	if err != nil {
		t.Fatalf("ParseCarrierStats() error = %v", err)
	}

	if filter.Carrier != "UA" {
		t.Errorf("carrier = %q, want UA", filter.Carrier)
	}
}
