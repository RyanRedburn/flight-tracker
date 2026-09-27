package postgres

import (
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestBuildRouteWeatherStatsQueryOptionalFilters(t *testing.T) {
	query, args := buildRouteWeatherStatsQuery(store.RouteStatsFilter{
		Origin:       testAirportORD,
		Dest:         testAirportLAX,
		StartDate:    testStartDate,
		EndDate:      testEndDate,
		Carrier:      "UA",
		FlightNumber: "100",
		DaysOfWeek:   []int{1, 2},
	})

	if !strings.Contains(query, "carrier = $") {
		t.Fatal("expected carrier filter in query")
	}

	if !strings.Contains(query, "flight_number::text") {
		t.Fatal("expected flight number filter in query")
	}

	if !strings.Contains(query, "day_of_week = ANY") {
		t.Fatal("expected days_of_week filter in query")
	}

	if strings.Contains(query, weatherStatsExtraPlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if len(args) != 7 {
		t.Fatalf("len(args) = %d, want 7", len(args))
	}

	if args[4] != "UA" || args[5] != "100" {
		t.Fatalf("filter args = %#v", args[4:])
	}
}

func TestBuildRouteWeatherStatsQueryCarrierOmitted(t *testing.T) {
	query, args := buildRouteWeatherStatsQuery(store.RouteStatsFilter{
		Origin:    testAirportORD,
		Dest:      testAirportLAX,
		StartDate: testStartDate,
		EndDate:   testEndDate,
	})

	if strings.Contains(query, "carrier =") {
		t.Fatal("omitted carrier should sum every carrier")
	}

	if strings.Contains(query, weatherStatsExtraPlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if len(args) != 4 {
		t.Fatalf("len(args) = %d, want 4", len(args))
	}
}
