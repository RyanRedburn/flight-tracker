package iem

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestIntersectStations(t *testing.T) {
	iemIDs := map[string]struct{}{
		testStationORD: {},
		testStationJFK: {},
		testStationATL: {},
	}

	matched, unmatched := intersectStations([]string{"ord", testStationXYZ, testStationJFK, "ord"}, iemIDs)
	if len(matched) != 2 || matched[0] != testStationJFK || matched[1] != testStationORD {
		t.Fatalf("matched = %v, want [JFK ORD]", matched)
	}

	if len(unmatched) != 1 || unmatched[0] != testStationXYZ {
		t.Fatalf("unmatched = %v, want [XYZ]", unmatched)
	}
}

func TestStationResolverResolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		network := strings.TrimSuffix(path.Base(r.URL.Path), ".geojson")
		features := []map[string]any{}

		switch network {
		case testNetworkILASOS:
			features = append(features, map[string]any{
				testGeoJSONProperties: map[string]any{testGeoJSONSID: testStationORD},
			})
		case testNetworkNYASOS:
			features = append(features, map[string]any{
				testGeoJSONProperties: map[string]any{testGeoJSONSID: testStationJFK},
			})
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"features": features})
	}))
	defer server.Close()

	st := &storetest.Stub{
		DistinctFlightAirportCodesFn: func(context.Context) ([]string, error) {
			return []string{testStationORD, testStationXYZ, testStationJFK}, nil
		},
	}

	catalog := NewNetworkCatalog(server.URL, 0)
	catalog.networks = []string{testNetworkILASOS, testNetworkNYASOS, "ZZ_ASOS"}

	resolver := NewStationResolver(st, catalog, slog.New(slog.NewTextHandler(io.Discard, nil)))

	stations, unmatched, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(stations) != 2 || stations[0] != testStationJFK || stations[1] != testStationORD {
		t.Fatalf("stations = %v, want [JFK ORD]", stations)
	}

	if len(unmatched) != 1 || unmatched[0] != testStationXYZ {
		t.Fatalf("unmatched = %v, want [XYZ]", unmatched)
	}
}

func TestStationResolverNoFlightAirports(t *testing.T) {
	st := &storetest.Stub{
		DistinctFlightAirportCodesFn: func(context.Context) ([]string, error) {
			return nil, nil
		},
	}

	resolver := NewStationResolver(st, NewNetworkCatalog("https://example.test", 0), slog.Default())

	_, _, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrNoFlightAirports) {
		t.Fatalf("Resolve() error = %v, want ErrNoFlightAirports", err)
	}
}

func TestNetworkCatalogSkips404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MISSING_ASOS") {
			http.NotFound(w, r)

			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"features": []map[string]any{
				{testGeoJSONProperties: map[string]any{testGeoJSONSID: testStationORD}},
			},
		})
	}))
	defer server.Close()

	catalog := NewNetworkCatalog(server.URL, 0)
	catalog.networks = []string{testNetworkILASOS, "MISSING_ASOS"}

	ids, err := catalog.LoadStationIDs(context.Background())
	if err != nil {
		t.Fatalf("LoadStationIDs() error = %v", err)
	}

	if _, ok := ids[testStationORD]; !ok {
		t.Fatalf("ids = %v, want ORD", ids)
	}
}
