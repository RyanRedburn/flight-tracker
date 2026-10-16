package postgres

import (
	"reflect"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestUnmarshalCarrierRoutesMatchesCountedRates(t *testing.T) {
	empty, err := unmarshalCarrierRoutes(nil)
	if err != nil {
		t.Fatalf("unmarshalCarrierRoutes() error = %v", err)
	}

	if empty == nil || len(empty) != 0 {
		t.Fatalf("nil json = %#v, want empty slice", empty)
	}

	empty, err = unmarshalCarrierRoutes([]byte(`null`))
	if err != nil {
		t.Fatalf("unmarshalCarrierRoutes() error = %v", err)
	}

	if len(empty) != 0 {
		t.Fatalf("null json len = %d", len(empty))
	}

	if _, err := unmarshalCarrierRoutes([]byte(`{`)); err == nil {
		t.Fatal("expected invalid json to fail")
	}

	raw := []byte(`[
		{"origin":"AAA","dest":"BBB","flights":40,"on_time":32,"delayed":4,"cancelled":2,"diverted":2},
		{"origin":"CCC","dest":"DDD","flights":38,"on_time":20,"delayed":10,"cancelled":4,"diverted":4},
		{"origin":"EEE","dest":"FFF","flights":36,"on_time":18,"delayed":10,"cancelled":4,"diverted":4},
		{"origin":"GGG","dest":"HHH","flights":34,"on_time":10,"delayed":14,"cancelled":5,"diverted":5},
		{"origin":"III","dest":"JJJ","flights":33,"on_time":8,"delayed":15,"cancelled":5,"diverted":5},
		{"origin":"KKK","dest":"LLL","flights":32,"on_time":6,"delayed":16,"cancelled":5,"diverted":5},
		{"origin":"QQQ","dest":"RRR","flights":29,"on_time":29,"delayed":0,"cancelled":0,"diverted":0}
	]`)

	routes, err := unmarshalCarrierRoutes(raw)
	if err != nil {
		t.Fatalf("unmarshalCarrierRoutes() error = %v", err)
	}

	built := make([]model.CarrierRouteStat, len(routes))
	for i, row := range routes {
		built[i] = model.CarrierRouteStat{
			Origin:           row.Origin,
			Dest:             row.Dest,
			Flights:          row.Flights,
			OnTime:           row.OnTime,
			Delayed:          row.Delayed,
			Cancelled:        row.Cancelled,
			Diverted:         row.Diverted,
			OnTimeRate:       ratio(row.OnTime, row.Flights),
			DelayRate:        ratio(row.Delayed, row.Flights),
			CancellationRate: ratio(row.Cancelled, row.Flights),
			DiversionRate:    ratio(row.Diverted, row.Flights),
		}
	}

	if !reflect.DeepEqual(routes, built) {
		t.Fatalf("routes = %+v, want rates from counts %+v", routes, built)
	}

	best, worst := store.RankCarrierRoutes(routes)
	wantBest, wantWorst := store.RankCarrierRoutes(built)

	if !reflect.DeepEqual(best, wantBest) || !reflect.DeepEqual(worst, wantWorst) {
		t.Fatalf("best=%+v worst=%+v", best, worst)
	}

	if len(best) != store.CarrierStatsTopN || len(worst) != 1 {
		t.Fatalf("best=%d worst=%d", len(best), len(worst))
	}

	for _, row := range append(append([]model.CarrierRouteStat{}, best...), worst...) {
		if row.Origin == "QQQ" {
			t.Fatal("29-flight route should not be ranked")
		}
	}
}

func TestUnmarshalCarrierAirportsMatchesCountedRates(t *testing.T) {
	empty, err := unmarshalCarrierAirports([]byte(`[]`))
	if err != nil {
		t.Fatalf("unmarshalCarrierAirports() error = %v", err)
	}

	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty json = %#v", empty)
	}

	if _, err := unmarshalCarrierAirports([]byte(`[`)); err == nil {
		t.Fatal("expected invalid json to fail")
	}

	raw := []byte(`[{"airport":"AAA","flights":70,"on_time":47,"delayed":14,"cancelled":5,"diverted":4}]`)

	airports, err := unmarshalCarrierAirports(raw)
	if err != nil {
		t.Fatalf("unmarshalCarrierAirports() error = %v", err)
	}

	if len(airports) != 1 {
		t.Fatalf("len = %d", len(airports))
	}

	got := airports[0]
	if got.Airport != "AAA" || got.Flights != 70 || got.OnTime != 47 || got.OnTimeRate != ratio(47, 70) || got.DelayRate != ratio(14, 70) || got.CancellationRate != ratio(5, 70) || got.DiversionRate != ratio(4, 70) {
		t.Fatalf("airport = %+v", got)
	}

	best, worst := store.RankCarrierAirports(airports)
	if len(best) != 1 || best[0].Airport != "AAA" || len(worst) != 0 {
		t.Fatalf("best=%+v worst=%+v", best, worst)
	}
}
