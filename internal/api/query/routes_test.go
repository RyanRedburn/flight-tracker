package query

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestParseRouteStats(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ord&dest=lax&start_date=2025-01-01&end_date=2025-01-31&carrier=ua&flight_number=100&days_of_week=1,2,2,3", nil)

	filter, err := ParseRouteStats(req)
	if err != nil {
		t.Fatalf("ParseRouteStats() error = %v", err)
	}

	if filter.Origin != "ORD" || filter.Dest != "LAX" || filter.Carrier != "UA" {
		t.Fatalf("normalized filter = %+v", filter)
	}

	if filter.FlightNumber != "100" {
		t.Errorf("flight_number = %q, want 100", filter.FlightNumber)
	}

	if len(filter.DaysOfWeek) != 3 {
		t.Fatalf("days_of_week = %v, want 3 unique days", filter.DaysOfWeek)
	}
}

func TestParseRouteStatsValidation(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"missing origin", "/api/v1/routes/stats?dest=LAX&start_date=2025-01-01&end_date=2025-01-02"},
		{"flight number without carrier", "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2025-01-01&end_date=2025-01-02&flight_number=100"},
		{"end before start", "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2025-02-01&end_date=2025-01-01"},
		{"span too long", "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2024-01-01&end_date=2025-12-31"},
		{"bad days", "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2025-01-01&end_date=2025-01-02&days_of_week=8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if _, err := ParseRouteStats(req); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseRouteOutlook(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ord&dest=lax&carrier=ua&day_of_week=2&dep_time=700", nil)

	filter, err := ParseRouteOutlook(req)
	if err != nil {
		t.Fatalf("ParseRouteOutlook() error = %v", err)
	}

	if filter.Origin != "ORD" || filter.Carrier != "UA" {
		t.Fatalf("normalized filter = %+v", filter)
	}

	if filter.DepTime != "0700" {
		t.Errorf("dep_time = %q, want 0700", filter.DepTime)
	}

	if filter.DepTimeWindowMinutes != store.DefaultDepTimeWindowMinutes {
		t.Errorf("window = %d, want %d", filter.DepTimeWindowMinutes, store.DefaultDepTimeWindowMinutes)
	}
}

func TestParseRouteOutlookWindowAndErrors(t *testing.T) {
	ok := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700&dep_time_window_minutes=45", nil)

	filter, err := ParseRouteOutlook(ok)
	if err != nil {
		t.Fatalf("ParseRouteOutlook() error = %v", err)
	}

	if filter.DepTimeWindowMinutes != 45 {
		t.Errorf("window = %d, want 45", filter.DepTimeWindowMinutes)
	}

	tests := []string{
		"/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=8&dep_time=0700",
		"/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=2500",
		"/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700&dep_time_window_minutes=0",
		"/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700&dep_time_window_minutes=121",
	}
	for _, url := range tests {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		if _, err := ParseRouteOutlook(req); err == nil {
			t.Fatalf("expected error for %s", url)
		}
	}
}
