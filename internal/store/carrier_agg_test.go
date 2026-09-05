package store

import (
	"strconv"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestDelayCausesShareFromCounts(t *testing.T) {
	share := DelayCausesShareFromCounts(0, 1, 0, 0, 0, 0, 0)
	if share != (model.DelayCausesShare{}) {
		t.Errorf("zero delayed = %+v, want zeros", share)
	}

	share = DelayCausesShareFromCounts(4, 0, 1, 0, 0, 2, 1)
	if share.LateAircraft != 0.5 || share.Weather != 0.25 || share.Unattributed != 0.25 || share.Carrier != 0 {
		t.Errorf("share = %+v", share)
	}
}

func TestCarrierStatsWindow(t *testing.T) {
	start, end, ok := CarrierStatsWindow(testFlightDate20260401)
	if !ok {
		t.Fatal("CarrierStatsWindow failed")
	}

	if end != testFlightDate20260401 {
		t.Errorf("end = %q, want %s", end, testFlightDate20260401)
	}

	if start != "2026-01-02" {
		t.Errorf("start = %q, want 2026-01-02 (90 inclusive days)", start)
	}

	if _, _, ok := CarrierStatsWindow("not-a-date"); ok {
		t.Fatal("expected invalid date to fail")
	}
}

func TestRankCarrierRoutes(t *testing.T) {
	items := make([]model.CarrierRouteStat, 0, 9)

	for i := range 8 {
		origin := "A" + strconv.Itoa(i)
		onTime := 30 - i
		items = append(items, model.CarrierRouteStat{
			Origin:     origin,
			Dest:       testAirportLAX,
			Flights:    CarrierStatsMinSampleSize,
			OnTime:     onTime,
			Delayed:    i,
			OnTimeRate: float64(onTime) / float64(CarrierStatsMinSampleSize),
		})
	}

	items = append(items, model.CarrierRouteStat{
		Origin: "ZZZ", Dest: testAirportLAX, Flights: 29, OnTime: 29, OnTimeRate: 1,
	})

	best, worst := RankCarrierRoutes(items)

	if len(best) != CarrierStatsTopN {
		t.Fatalf("best len = %d, want %d", len(best), CarrierStatsTopN)
	}

	if len(worst) != 3 {
		t.Fatalf("worst len = %d, want 3 (8 qualifying minus best 5)", len(worst))
	}

	if best[0].Origin != "A0" || best[4].Origin != "A4" {
		t.Errorf("best origins = %s .. %s, want A0 .. A4", best[0].Origin, best[4].Origin)
	}

	if worst[0].Origin != "A7" {
		t.Errorf("worst[0] = %s, want A7", worst[0].Origin)
	}

	for _, route := range best {
		if route.Origin == "ZZZ" {
			t.Fatal("route with 29 flights should not qualify")
		}

		for _, w := range worst {
			if route.Origin == w.Origin && route.Dest == w.Dest {
				t.Fatalf("route %s-%s in both best and worst", route.Origin, route.Dest)
			}
		}
	}
}

func TestRankCarrierRoutesFewQualify(t *testing.T) {
	best, worst := RankCarrierRoutes([]model.CarrierRouteStat{
		{Origin: "AAA", Dest: "BBB", Flights: CarrierStatsMinSampleSize, OnTimeRate: 1},
		{Origin: "CCC", Dest: "DDD", Flights: CarrierStatsMinSampleSize, OnTimeRate: 1},
		{Origin: "EEE", Dest: "FFF", Flights: CarrierStatsMinSampleSize, OnTimeRate: 0},
	})

	if len(best) != 3 {
		t.Fatalf("best len = %d, want 3", len(best))
	}

	if len(worst) != 0 {
		t.Fatalf("worst len = %d, want 0 when all fit in best", len(worst))
	}
}

func TestRankCarrierAirportsFewQualify(t *testing.T) {
	best, worst := RankCarrierAirports([]model.CarrierAirportStat{
		{Airport: "BOS", Flights: CarrierStatsMinSampleSize, OnTimeRate: 1},
		{Airport: "DEN", Flights: CarrierStatsMinSampleSize, OnTimeRate: 1},
		{Airport: "JFK", Flights: CarrierStatsMinSampleSize, OnTimeRate: 1},
		{Airport: "MIA", Flights: CarrierStatsMinSampleSize, OnTimeRate: 1},
		{Airport: "SEA", Flights: CarrierStatsMinSampleSize, OnTimeRate: 0},
		{Airport: "PHX", Flights: CarrierStatsMinSampleSize, OnTimeRate: 0},
	})

	if len(best) != 5 {
		t.Fatalf("best len = %d, want 5", len(best))
	}

	if len(worst) != 1 {
		t.Fatalf("worst len = %d, want 1", len(worst))
	}

	if best[0].Airport != "BOS" || worst[0].Airport != "SEA" {
		t.Errorf("best[0]=%s worst[0]=%s, want BOS and SEA", best[0].Airport, worst[0].Airport)
	}
}

func TestRankCarrierEmpty(t *testing.T) {
	bestR, worstR := RankCarrierRoutes(nil)
	bestA, worstA := RankCarrierAirports(nil)

	if bestR == nil || worstR == nil || bestA == nil || worstA == nil {
		t.Fatal("ranked lists must be empty slices, not nil")
	}
}

func TestCarrierStatsMinSampleSQLMatchesConstant(t *testing.T) {
	want := strconv.Itoa(CarrierStatsMinSampleSize)
	if !strings.Contains(QueryCarrierStatsRoutes, "HAVING COUNT(*) >= "+want) {
		t.Errorf("QueryCarrierStatsRoutes missing HAVING >= %s", want)
	}

	if !strings.Contains(QueryCarrierStatsAirports, "HAVING COUNT(*) >= "+want) {
		t.Errorf("QueryCarrierStatsAirports missing HAVING >= %s", want)
	}
}
