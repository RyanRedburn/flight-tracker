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

func TestCarrierOnTimeRateFromCountsMatchesAggregateOnTime(t *testing.T) {
	// Same classification as QueryRouteStats / QueryRouteStatsCarrierOnTimeRates:
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
		flights, onTime, delayed, cancelled, diverted int
	}

	var agg counts

	byCarrier := map[string]counts{}
	for _, f := range flights {
		onTime, delayed, cancelled, diverted := classifyRouteStatsFlight(f.cancelled, f.diverted, f.arrDel15)
		agg.flights++
		agg.onTime += onTime
		agg.delayed += delayed
		agg.cancelled += cancelled
		agg.diverted += diverted

		cur := byCarrier[f.carrier]
		cur.flights++
		cur.onTime += onTime
		cur.delayed += delayed
		cur.cancelled += cancelled
		cur.diverted += diverted
		byCarrier[f.carrier] = cur
	}

	aggRate := rate(agg.onTime, agg.flights)
	if aggRate != rate(2, 6) {
		t.Fatalf("aggregate on_time_rate = %v, want 2/6 (not cancelled/diverted, arr_del15 < 1)", aggRate)
	}

	var (
		items                    []model.CarrierOnTimeRate
		sumFlights, sumOnTime    int
		sumDelayed, sumCancelled int
		sumDiverted              int
	)

	for carrier, c := range byCarrier {
		item := CarrierOnTimeRateFromCounts(carrier, c.flights, c.onTime, c.delayed, c.cancelled, c.diverted)
		if item.OnTimeRate != rate(c.onTime, c.flights) {
			t.Errorf("%s on_time_rate = %v, want on_time/flights", carrier, item.OnTimeRate)
		}

		items = append(items, item)
		sumFlights += item.Flights
		sumOnTime += item.OnTime
		sumDelayed += item.Delayed
		sumCancelled += item.Cancelled
		sumDiverted += item.Diverted
	}

	if sumFlights != agg.flights || sumOnTime != agg.onTime || sumDelayed != agg.delayed || sumCancelled != agg.cancelled || sumDiverted != agg.diverted {
		t.Fatalf("per-carrier counts do not partition aggregate: summed=%d/%d/%d/%d/%d agg=%+v",
			sumFlights, sumOnTime, sumDelayed, sumCancelled, sumDiverted, agg)
	}

	if rate(sumOnTime, sumFlights) != aggRate {
		t.Fatalf("partitioned on_time_rate = %v, want aggregate %v", rate(sumOnTime, sumFlights), aggRate)
	}

	SortCarrierOnTimeRates(items)

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

func TestSortCarrierOnTimeRates(t *testing.T) {
	items := []model.CarrierOnTimeRate{
		{Carrier: "UA", Flights: 5, OnTimeRate: 0.5},
		{Carrier: "B6", Flights: 5, OnTimeRate: 0.5},
		{Carrier: "AA", Flights: 10, OnTimeRate: 0.5},
		{Carrier: "DL", Flights: 1, OnTimeRate: 1},
	}

	SortCarrierOnTimeRates(items)

	want := []string{"DL", "AA", "B6", "UA"}
	for i, carrier := range want {
		if items[i].Carrier != carrier {
			t.Errorf("items[%d].Carrier = %q, want %q", i, items[i].Carrier, carrier)
		}
	}
}

func TestCarrierOnTimeRateFromCountsZeroFlights(t *testing.T) {
	got := CarrierOnTimeRateFromCounts("UA", 0, 0, 0, 0, 0)
	if got.OnTimeRate != 0 || got.DelayRate != 0 || got.CancellationRate != 0 || got.DiversionRate != 0 {
		t.Fatalf("zero flights rates = %+v, want zeros", got)
	}
}

// classifyRouteStatsFlight mirrors the SQL in QueryRouteStats and
// QueryRouteStatsCarrierOnTimeRates: cancelled takes priority, then diverted,
// then delayed when arr_del15 >= 1. On-time is the remaining operated flights.
func classifyRouteStatsFlight(cancelled, diverted, arrDel15 float64) (onTime, delayed, isCancelled, isDiverted int) {
	switch {
	case cancelled >= 1:
		return 0, 0, 1, 0
	case diverted >= 1:
		return 0, 0, 0, 1
	case arrDel15 >= 1:
		return 0, 1, 0, 0
	default:
		return 1, 0, 0, 0
	}
}
