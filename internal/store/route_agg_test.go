package store

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	testFlightDate20260401 = "2026-04-01"
	testAirportLAX         = "LAX"
)

func TestHHMMToMinutes(t *testing.T) {
	tests := []struct {
		raw    string
		want   int
		wantOK bool
	}{
		{"0700", 420, true},
		{"700", 420, true},
		{"2350", 1430, true},
		{"2400", 0, false},
		{"ab", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := HHMMToMinutes(tt.raw)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("HHMMToMinutes(%q) = (%d,%v), want (%d,%v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestCarrierOnTimeMatchesAggregateOnTime(t *testing.T) {
	// Same classification as QueryRouteStats / QueryRouteStatsCarrierOnTime:
	// cancelled, else diverted, else delayed when arr_del15 >= 1, else on-time.
	type flight struct {
		carrier                       string
		cancelled, diverted, arrDel15 float64
	}

	flights := []flight{
		{carrier: "UA", cancelled: 0, diverted: 0, arrDel15: 0},
		{carrier: "UA", cancelled: 0, diverted: 0, arrDel15: 1},
		{carrier: "UA", cancelled: 1, diverted: 0, arrDel15: 0},
		{carrier: "AA", cancelled: 0, diverted: 0, arrDel15: 0},
		{carrier: "AA", cancelled: 0, diverted: 1, arrDel15: 0},
		{carrier: "DL", cancelled: 0, diverted: 0, arrDel15: 0.5},
	}

	type counts struct {
		flights, onTime int
	}

	var agg counts

	byCarrier := map[string]counts{}

	for _, f := range flights {
		onTime := classifyRouteStatsOnTime(f.cancelled, f.diverted, f.arrDel15)
		agg.flights++
		agg.onTime += onTime

		cur := byCarrier[f.carrier]
		cur.flights++
		cur.onTime += onTime
		byCarrier[f.carrier] = cur
	}

	if agg.onTime != 3 || agg.flights != 6 {
		t.Fatalf("aggregate on_time/flights = %d/%d, want 3/6 (not cancelled/diverted, arr_del15 < 1)", agg.onTime, agg.flights)
	}

	aggRate := rate(agg.onTime, agg.flights)

	var (
		items      []model.CarrierOnTime
		sumFlights int
	)

	for carrier, c := range byCarrier {
		item := CarrierOnTimeFromCounts(carrier, c.onTime, c.flights)
		if item.OnTimeRate != rate(c.onTime, c.flights) {
			t.Errorf("%s on_time_rate = %v, want on_time_count/flights", carrier, item.OnTimeRate)
		}

		items = append(items, item)
		sumFlights += item.Flights
	}

	if sumFlights != agg.flights {
		t.Fatalf("per-carrier flights %d do not partition aggregate %d", sumFlights, agg.flights)
	}

	if aggRate != rate(3, 6) {
		t.Fatalf("aggregate on_time_rate = %v, want 3/6", aggRate)
	}

	SortCarrierOnTime(items)

	wantOrder := []string{"DL", "AA", "UA"}
	if len(items) != len(wantOrder) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(wantOrder))
	}

	for i, carrier := range wantOrder {
		if items[i].Carrier != carrier {
			t.Errorf("items[%d].Carrier = %q, want %q", i, items[i].Carrier, carrier)
		}
	}
}

func TestSortCarrierOnTime(t *testing.T) {
	items := []model.CarrierOnTime{
		{Carrier: "UA", OnTimeRate: 0.5, Flights: 2},
		{Carrier: "B6", OnTimeRate: 0.5, Flights: 2},
		{Carrier: "AA", OnTimeRate: 0.5, Flights: 10},
		{Carrier: "DL", OnTimeRate: 1, Flights: 1},
	}

	SortCarrierOnTime(items)

	want := []string{"DL", "AA", "B6", "UA"}
	for i, carrier := range want {
		if items[i].Carrier != carrier {
			t.Errorf("items[%d].Carrier = %q, want %q", i, items[i].Carrier, carrier)
		}
	}
}

// classifyRouteStatsOnTime reports 1 when the flight is on time: not cancelled,
// then not diverted, then arr_del15 < 1. Cancelled takes priority over diverted,
// and diverted takes priority over delayed.
func classifyRouteStatsOnTime(cancelled, diverted, arrDel15 float64) int {
	switch {
	case cancelled >= 1:
		return 0
	case diverted >= 1:
		return 0
	case arrDel15 >= 1:
		return 0
	default:
		return 1
	}
}
