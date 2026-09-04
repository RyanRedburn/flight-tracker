package iem

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestMapFlightAirports(t *testing.T) {
	catalog := map[string]Station{
		testStationORD: {SID: testStationORD, TzName: testTzChicago},
		testStationJFK: {SID: testStationJFK, TzName: "America/New_York"},
		testStationATL: {SID: testStationATL, TzName: "America/New_York"},
	}

	rows := mapFlightAirports([]string{"ord", testStationXYZ, testStationJFK, "ord"}, catalog, nil)
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}

	if rows[0].AirportCode != testStationJFK || !rows[0].Matched || rows[0].IEMSID != testStationJFK {
		t.Fatalf("row[0] = %+v, want matched JFK", rows[0])
	}

	if rows[1].AirportCode != testStationORD || !rows[1].Matched || rows[1].TzName != testTzChicago {
		t.Fatalf("row[1] = %+v, want matched ORD", rows[1])
	}

	if rows[2].AirportCode != testStationXYZ || rows[2].Matched || rows[2].IEMSID != "" || rows[2].TzName != "" {
		t.Fatalf("row[2] = %+v, want unmatched XYZ", rows[2])
	}
}

func TestMapFlightAirportsICAOFallback(t *testing.T) {
	catalog := map[string]Station{
		testStationPHNL: {SID: testStationPHNL, TzName: testTzHonolulu},
	}
	refs := map[string]store.AirportIdentifiers{
		testStationHNL: {IATACode: testStationHNL, LocalCode: testStationHNL, ICAOCode: testStationPHNL, Ident: testStationPHNL},
	}

	rows := mapFlightAirports([]string{testStationHNL}, catalog, refs)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}

	if !rows[0].Matched || rows[0].IEMSID != testStationPHNL || rows[0].TzName != testTzHonolulu {
		t.Fatalf("row = %+v, want matched PHNL", rows[0])
	}
}

func TestMapFlightAirportsFAAFallback(t *testing.T) {
	catalog := map[string]Station{
		testStationIWA: {SID: testStationIWA, TzName: testTzPhoenix},
	}
	refs := map[string]store.AirportIdentifiers{
		testStationAZA: {IATACode: testStationAZA, LocalCode: testStationIWA, ICAOCode: testStationKIWA, Ident: testStationKIWA},
	}

	rows := mapFlightAirports([]string{testStationAZA}, catalog, refs)
	if len(rows) != 1 || !rows[0].Matched || rows[0].IEMSID != testStationIWA {
		t.Fatalf("row = %+v, want matched IWA", rows)
	}
}

func TestMapFlightAirportsPrefersIATAOverFAA(t *testing.T) {
	catalog := map[string]Station{
		testStationYUM: {SID: testStationYUM, TzName: testTzPhoenix},
		testStationNYL: {SID: testStationNYL, TzName: testTzPhoenix},
	}
	refs := map[string]store.AirportIdentifiers{
		testStationYUM: {IATACode: testStationYUM, LocalCode: testStationNYL, ICAOCode: "KNYL", Ident: "KNYL"},
	}

	rows := mapFlightAirports([]string{testStationYUM}, catalog, refs)
	if len(rows) != 1 || !rows[0].Matched || rows[0].IEMSID != testStationYUM {
		t.Fatalf("row = %+v, want matched YUM", rows)
	}
}

func TestMapFlightAirportsEmptyRefsExactOnly(t *testing.T) {
	catalog := map[string]Station{
		testStationPHNL: {SID: testStationPHNL, TzName: testTzHonolulu},
		testStationORD:  {SID: testStationORD, TzName: testTzChicago},
	}

	rows := mapFlightAirports([]string{testStationHNL, testStationORD}, catalog, nil)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	if rows[0].AirportCode != testStationHNL || rows[0].Matched {
		t.Fatalf("row[0] = %+v, want unmatched HNL", rows[0])
	}

	if rows[1].AirportCode != testStationORD || !rows[1].Matched || rows[1].IEMSID != testStationORD {
		t.Fatalf("row[1] = %+v, want matched ORD", rows[1])
	}
}

func TestMapFlightAirportsNoCandidateHit(t *testing.T) {
	catalog := map[string]Station{
		testStationORD: {SID: testStationORD, TzName: testTzChicago},
	}
	refs := map[string]store.AirportIdentifiers{
		testStationHNL: {IATACode: testStationHNL, LocalCode: testStationHNL, ICAOCode: testStationPHNL, Ident: testStationPHNL},
	}

	rows := mapFlightAirports([]string{testStationHNL}, catalog, refs)
	if len(rows) != 1 || rows[0].Matched || rows[0].IEMSID != "" {
		t.Fatalf("row = %+v, want unmatched HNL", rows)
	}
}

func TestStationCandidatesOrder(t *testing.T) {
	refs := map[string]store.AirportIdentifiers{
		testStationAZA: {LocalCode: testStationIWA, ICAOCode: testStationKIWA, Ident: testStationKIWA},
	}

	got := stationCandidates(testStationAZA, refs)
	want := []string{testStationAZA, testStationIWA, testStationKIWA}

	if len(got) != len(want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidates = %v, want %v", got, want)
		}
	}
}

func TestLoadStationsParsesProperties(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, testNetworkILASOS) {
			http.NotFound(w, r)

			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			testGeoJSONFeatures: []map[string]any{
				{
					testGeoJSONProperties: map[string]any{
						testGeoJSONSID:          testStationORD,
						testGeoJSONNetwork:      testNetworkILASOS,
						testGeoJSONSName:        testORDName,
						testGeoJSONTzName:       testTzChicago,
						testGeoJSONArchiveBegin: testORDArchiveBegin,
					},
					testGeoJSONGeometry: map[string]any{
						testGeoJSONCoordinates: []float64{testORDLongitude, testORDLatitude},
					},
				},
			},
		})
	}))
	defer server.Close()

	catalog := NewNetworkCatalog(server.URL, 0)
	catalog.networks = []string{testNetworkILASOS, "MISSING_ASOS"}

	stations, err := catalog.LoadStations(context.Background())
	if err != nil {
		t.Fatalf("LoadStations() error = %v", err)
	}

	station, ok := stations[testStationORD]
	if !ok {
		t.Fatalf("stations = %v, want ORD", stations)
	}

	if station.Network != testNetworkILASOS || station.Name != testORDName || station.TzName != testTzChicago {
		t.Fatalf("station = %+v", station)
	}

	if station.ArchiveBegin != testORDArchiveBegin {
		t.Fatalf("archive_begin = %q, want %q", station.ArchiveBegin, testORDArchiveBegin)
	}

	if station.Longitude == nil || station.Latitude == nil {
		t.Fatal("expected coordinates")
	}

	if *station.Longitude != testORDLongitude || *station.Latitude != testORDLatitude {
		t.Fatalf("coords = (%v,%v), want (%v,%v)", *station.Longitude, *station.Latitude, testORDLongitude, testORDLatitude)
	}
}

func TestStationResolverResolve(t *testing.T) {
	st := &storetest.Stub{
		ListAirportWeatherStationsFn: func(context.Context) ([]store.AirportWeatherStation, error) {
			return []store.AirportWeatherStation{
				{AirportCode: testStationORD, IEMSID: testStationORD, TzName: testTzChicago, Matched: true},
				{AirportCode: testStationXYZ, Matched: false},
				{AirportCode: testStationJFK, IEMSID: testStationJFK, Matched: true},
			}, nil
		},
	}

	resolver := NewStationResolver(st, slog.New(slog.NewTextHandler(io.Discard, nil)))

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

func TestStationResolverNoMapping(t *testing.T) {
	st := &storetest.Stub{
		ListAirportWeatherStationsFn: func(context.Context) ([]store.AirportWeatherStation, error) {
			return nil, nil
		},
	}

	resolver := NewStationResolver(st, slog.Default())

	_, _, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrNoWeatherStationMapping) {
		t.Fatalf("Resolve() error = %v, want ErrNoWeatherStationMapping", err)
	}
}

func TestStationResolverNoMatchedStations(t *testing.T) {
	st := &storetest.Stub{
		ListAirportWeatherStationsFn: func(context.Context) ([]store.AirportWeatherStation, error) {
			return []store.AirportWeatherStation{
				{AirportCode: testStationXYZ, Matched: false},
			}, nil
		},
	}

	resolver := NewStationResolver(st, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, unmatched, err := resolver.Resolve(context.Background())
	if !errors.Is(err, ErrNoMatchedStations) {
		t.Fatalf("Resolve() error = %v, want ErrNoMatchedStations", err)
	}

	if len(unmatched) != 1 || unmatched[0] != testStationXYZ {
		t.Fatalf("unmatched = %v, want [XYZ]", unmatched)
	}
}

func TestNetworkCatalogSkips404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MISSING_ASOS") {
			http.NotFound(w, r)

			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			testGeoJSONFeatures: []map[string]any{
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

func TestNetworkCatalogUsesFallbackNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testGeoJSONFeatures: []map[string]any{
				{
					testGeoJSONProperties: map[string]any{
						testGeoJSONSID: testStationORD,
					},
				},
			},
		})
	}))
	defer server.Close()

	catalog := NewNetworkCatalog(server.URL, 0)
	catalog.networks = []string{testNetworkILASOS}

	stations, err := catalog.LoadStations(context.Background())
	if err != nil {
		t.Fatalf("LoadStations() error = %v", err)
	}

	if stations[testStationORD].Network != testNetworkILASOS {
		t.Fatalf("network = %q, want %q", stations[testStationORD].Network, testNetworkILASOS)
	}
}
