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
		{"origin":"AAA","dest":"BBB","flights":140,"on_time":112,"delayed":14,"cancelled":7,"diverted":7},
		{"origin":"CCC","dest":"DDD","flights":138,"on_time":70,"delayed":36,"cancelled":16,"diverted":16},
		{"origin":"EEE","dest":"FFF","flights":136,"on_time":68,"delayed":36,"cancelled":16,"diverted":16},
		{"origin":"GGG","dest":"HHH","flights":134,"on_time":40,"delayed":54,"cancelled":20,"diverted":20},
		{"origin":"III","dest":"JJJ","flights":133,"on_time":32,"delayed":61,"cancelled":20,"diverted":20},
		{"origin":"KKK","dest":"LLL","flights":132,"on_time":24,"delayed":64,"cancelled":22,"diverted":22},
		{"origin":"QQQ","dest":"RRR","flights":99,"on_time":99,"delayed":0,"cancelled":0,"diverted":0}
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
			t.Fatal("99-flight route should not be ranked")
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

	raw := []byte(`[{"airport":"AAA","flights":100,"on_time":47,"delayed":14,"cancelled":5,"diverted":4}]`)

	airports, err := unmarshalCarrierAirports(raw)
	if err != nil {
		t.Fatalf("unmarshalCarrierAirports() error = %v", err)
	}

	if len(airports) != 1 {
		t.Fatalf("len = %d", len(airports))
	}

	got := airports[0]
	if got.Airport != "AAA" || got.Flights != 100 || got.OnTime != 47 || got.OnTimeRate != ratio(47, 100) || got.DelayRate != ratio(14, 100) || got.CancellationRate != ratio(5, 100) || got.DiversionRate != ratio(4, 100) {
		t.Fatalf("airport = %+v", got)
	}

	best, worst := store.RankCarrierAirports(airports)
	if len(best) != 1 || best[0].Airport != "AAA" || len(worst) != 0 {
		t.Fatalf("best=%+v worst=%+v", best, worst)
	}
}
