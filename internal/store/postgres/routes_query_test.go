package postgres

import (
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestToPgx5URL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "postgres://localhost:5432/flight_tracker?sslmode=disable", want: "pgx5://localhost:5432/flight_tracker?sslmode=disable"},
		{in: "postgresql://flight@localhost/db", want: "pgx5://flight@localhost/db"},
		{in: "pgx5://already", want: "pgx5://already"},
	}

	for _, tt := range tests {
		if got := toPgx5URL(tt.in); got != tt.want {
			t.Errorf("toPgx5URL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBuildRouteStatsQueryOptionalFilters(t *testing.T) {
	query, args := buildRouteStatsQuery(store.RouteStatsFilter{
		Origin:       testAirportORD,
		Dest:         testAirportLAX,
		StartDate:    testStartDate,
		EndDate:      testEndDate,
		Carrier:      "UA",
		FlightNumber: "100",
		DaysOfWeek:   []int{1, 2},
	})

	if !strings.Contains(query, "iata_code_marketing_airline") {
		t.Fatal("expected carrier filter in query")
	}

	if !strings.Contains(query, "flight_number_marketing_airline::text") {
		t.Fatal("expected flight number filter in query")
	}

	if !strings.Contains(query, "day_of_week = ANY") {
		t.Fatal("expected days_of_week filter in query")
	}

	if strings.Contains(query, routeStatsExtraPlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if len(args) != 7 {
		t.Fatalf("len(args) = %d, want 7", len(args))
	}
}

func TestBuildCarrierStatsQueryStateFilter(t *testing.T) {
	query, args := buildCarrierStatsQuery(store.QueryCarrierStats, store.CarrierStatsFilter{
		Carrier:   "UA",
		StartDate: testStartDate,
		EndDate:   testEndDate,
		State:     "IL",
	})

	if !strings.Contains(query, "origin_state = $4 OR dest_state = $4") {
		t.Fatal("expected state filter in query")
	}

	if strings.Contains(query, carrierStatsExtraPlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if len(args) != 4 {
		t.Fatalf("len(args) = %d, want 4", len(args))
	}

	noState, args := buildCarrierStatsQuery(store.QueryCarrierStats, store.CarrierStatsFilter{
		Carrier:   "UA",
		StartDate: testStartDate,
		EndDate:   testEndDate,
	})
	if strings.Contains(noState, "origin_state") {
		t.Fatal("state filter should be omitted")
	}

	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3", len(args))
	}
}

func TestUnmarshalAirportCountsEmpty(t *testing.T) {
	got, err := unmarshalAirportCounts(nil)
	if err != nil {
		t.Fatalf("unmarshalAirportCounts() error = %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Errorf("got = %#v, want empty slice", got)
	}

	got, err = unmarshalAirportCounts([]byte(`[{"airport":"` + testAirportMDW + `","count":2}]`))
	if err != nil {
		t.Fatalf("unmarshalAirportCounts() error = %v", err)
	}

	if len(got) != 1 || got[0] != (model.AirportCount{Airport: testAirportMDW, Count: 2}) {
		t.Errorf("got = %#v", got)
	}
}
