package postgres

import (
	"strconv"
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

func TestRouteIdentityQuery(t *testing.T) {
	query, args := routeIdentityQuery(testAirportORD, testAirportLAX, "")
	if query != store.QueryRouteIdentity {
		t.Fatal("omitted carrier should use origin/dest identity")
	}

	if len(args) != 2 || args[0] != testAirportORD || args[1] != testAirportLAX {
		t.Fatalf("args = %#v", args)
	}

	query, args = routeIdentityQuery(testAirportORD, testAirportLAX, "UA")
	if query != store.QueryRouteCarrierIdentity {
		t.Fatal("set carrier should use origin/dest/carrier identity")
	}

	if len(args) != 3 || args[2] != "UA" {
		t.Fatalf("args = %#v", args)
	}
}

func TestBuildRouteStatsQueryOptionalFilters(t *testing.T) {
	query, args := buildRouteStatsQuery(store.QueryRouteStats, store.RouteStatsFilter{
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

	if !strings.Contains(query, "flight_number_marketing_airline = $") {
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

func TestBuildRouteStatsQueryCarrierOnTime(t *testing.T) {
	unfiltered, args := buildRouteStatsQuery(store.QueryRouteStats, store.RouteStatsFilter{
		Origin:     testAirportORD,
		Dest:       testAirportLAX,
		StartDate:  testStartDate,
		EndDate:    testEndDate,
		DaysOfWeek: []int{1, 2},
	})

	if strings.Contains(unfiltered, "iata_code_marketing_airline =") {
		t.Fatal("unfiltered query should not constrain a single carrier")
	}

	if !strings.Contains(unfiltered, "json_build_object('carrier'") {
		t.Fatal("expected per-carrier on-time aggregate in the route stats query")
	}

	if !strings.Contains(unfiltered, "day_of_week = ANY") {
		t.Fatal("expected days_of_week filter")
	}

	if strings.Contains(unfiltered, routeStatsExtraPlaceholder) || strings.Contains(unfiltered, routeStatsCarrierOnTimePlaceholder) {
		t.Fatal("placeholders should be replaced")
	}

	if len(args) != 5 {
		t.Fatalf("len(args) = %d, want 5", len(args))
	}

	filtered, filteredArgs := buildRouteStatsQuery(store.QueryRouteStats, store.RouteStatsFilter{
		Origin:    testAirportORD,
		Dest:      testAirportLAX,
		StartDate: testStartDate,
		EndDate:   testEndDate,
		Carrier:   "UA",
	})

	if !strings.Contains(filtered, "NULL::json") {
		t.Fatal("carrier filter should skip the per-carrier aggregate")
	}

	if strings.Contains(filtered, "json_build_object('carrier'") {
		t.Fatal("carrier filter should not build per-carrier json")
	}

	if strings.Contains(filtered, routeStatsCarrierOnTimePlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if len(filteredArgs) != 5 || filteredArgs[4] != "UA" {
		t.Fatalf("args = %#v", filteredArgs)
	}
}

func TestBuildCarrierStatsQueryStateFilter(t *testing.T) {
	filter := store.CarrierStatsFilter{
		Carrier:   "UA",
		StartDate: testStartDate,
		EndDate:   testEndDate,
		State:     "IL",
	}

	query, args := buildCarrierStatsQuery(store.QueryCarrierStats, filter)
	if !strings.Contains(query, "origin_state = $4 OR dest_state = $4") {
		t.Fatal("expected state filter in query")
	}

	if strings.Contains(query, carrierStatsExtraPlaceholder) ||
		strings.Contains(query, carrierStatsOriginAirportPlaceholder) ||
		strings.Contains(query, carrierStatsDestAirportPlaceholder) ||
		strings.Contains(query, carrierStatsMinSamplePlaceholder) {
		t.Fatal("placeholders should be replaced")
	}

	wantHaving := "HAVING COUNT(*) >= " + strconv.Itoa(store.CarrierRankMinSampleSize)
	if strings.Count(query, wantHaving) != 2 {
		t.Fatalf("having clauses = %d, want 2", strings.Count(query, wantHaving))
	}

	if !strings.Contains(query, "WHERE origin_state = $4") {
		t.Fatal("expected ranked origins to be limited to the filtered state")
	}

	if !strings.Contains(query, "WHERE dest_state = $4") {
		t.Fatal("expected ranked dests to be limited to the filtered state")
	}

	if len(args) != 4 || args[3] != "IL" {
		t.Fatalf("args = %#v", args)
	}

	noStateFilter := store.CarrierStatsFilter{
		Carrier:   "UA",
		StartDate: testStartDate,
		EndDate:   testEndDate,
	}

	noState, args := buildCarrierStatsQuery(store.QueryCarrierStats, noStateFilter)
	if strings.Contains(noState, "origin_state = $4") || strings.Contains(noState, "dest_state = $4") {
		t.Fatal("state filter should be omitted")
	}

	if strings.Count(noState, "WHERE TRUE") != 2 {
		t.Fatal("expected unfiltered airport ranking")
	}

	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3", len(args))
	}
}

func TestUnmarshalCarrierOnTime(t *testing.T) {
	got, err := unmarshalCarrierOnTime(nil)
	if err != nil {
		t.Fatalf("unmarshalCarrierOnTime() error = %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("nil json = %#v, want empty slice", got)
	}

	got, err = unmarshalCarrierOnTime([]byte(`[{"carrier":"AA","on_time":1,"flights":4},{"carrier":"UA","on_time":5,"flights":6}]`))
	if err != nil {
		t.Fatalf("unmarshalCarrierOnTime() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	if got[0].Carrier != "UA" || got[0].Flights != 6 || got[0].OnTimeCount == nil || *got[0].OnTimeCount != 5 || got[0].OnTimeRate != store.CarrierOnTimeFromCounts("UA", 5, 6).OnTimeRate {
		t.Errorf("first = %+v", got[0])
	}

	if got[1].Carrier != "AA" || got[1].Flights != 4 {
		t.Errorf("second = %+v", got[1])
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
