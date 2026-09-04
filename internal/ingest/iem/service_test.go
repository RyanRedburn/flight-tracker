package iem

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestServiceImportMonth(t *testing.T) {
	ctx := context.Background()

	var gotYear, gotMonth int

	var gotColumns []string

	var gotRows [][]string

	st := &storetest.Stub{
		ReplaceWeatherObservationsByMonthFn: func(_ context.Context, year, month int, columns []string, rows [][]string) error {
			gotYear, gotMonth = year, month

			gotColumns = append([]string(nil), columns...)
			gotRows = rows

			return nil
		},
	}
	svc := NewService(st, nil).WithCSVOpener(minimalCSVOpener(t))

	result, err := svc.ImportMonth(ctx, 2024, 1, []string{testStationORD, testStationJFK})
	if err != nil {
		t.Fatalf("ImportMonth() error = %v", err)
	}

	if result.RowsImported != testdataRowCount {
		t.Fatalf("RowsImported = %d, want %d", result.RowsImported, testdataRowCount)
	}

	if result.Year != 2024 || result.Month != 1 {
		t.Errorf("result = %+v, want year 2024 month 1", result)
	}

	if gotYear != 2024 || gotMonth != 1 || len(gotRows) != testdataRowCount {
		t.Errorf("ReplaceWeatherObservationsByMonth = %d-%d rows=%d, want 2024-1 rows=%d",
			gotYear, gotMonth, len(gotRows), testdataRowCount)
	}

	if len(gotColumns) != len(DBColumns) {
		t.Fatalf("len(columns) = %d, want %d", len(gotColumns), len(DBColumns))
	}

	if gotColumns[0] != colYear || gotColumns[1] != colMonth {
		t.Errorf("partition columns = %q,%q, want year,month", gotColumns[0], gotColumns[1])
	}

	validIdx := -1

	for i, col := range gotColumns {
		if col == colValid {
			validIdx = i

			break
		}
	}

	if validIdx < 0 {
		t.Fatal("valid column missing from replace columns")
	}

	parsed, err := time.Parse(time.RFC3339, gotRows[0][validIdx])
	if err != nil {
		t.Fatalf("valid not RFC3339: %q (%v)", gotRows[0][validIdx], err)
	}

	if parsed.Location() != time.UTC {
		t.Errorf("valid location = %v, want UTC", parsed.Location())
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded[jsonKeyRows] == nil {
		t.Fatal("expected rows_imported in json result")
	}
}

func TestServiceImportMonthWithoutDownloader(t *testing.T) {
	svc := NewService(&storetest.Stub{}, nil)

	_, err := svc.ImportMonth(context.Background(), 2024, 1, []string{testStationORD})
	if err == nil {
		t.Fatal("ImportMonth() expected error without downloader or opener")
	}
}

func TestServiceImportMonthEmptyStations(t *testing.T) {
	svc := NewService(&storetest.Stub{}, nil).WithCSVOpener(minimalCSVOpener(t))

	_, err := svc.ImportMonth(context.Background(), 2024, 1, nil)
	if !errors.Is(err, ErrEmptyStations) {
		t.Fatalf("ImportMonth() error = %v, want ErrEmptyStations", err)
	}
}

func TestImportResultJSON(t *testing.T) {
	payload, err := ImportResult{Year: 2024, Month: 1, RowsImported: 42}.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded[jsonKeyRows] != float64(42) {
		t.Errorf("rows_imported = %v, want 42", decoded[jsonKeyRows])
	}
}

func TestDedupWeatherRowsLastWins(t *testing.T) {
	columns := []string{colYear, colMonth, colStation, colValid, colTmpf}
	rows := [][]string{
		{"2024", "1", testStationORD, testValidTimestamp, "32.00"},
		{"2024", "1", testStationJFK, testValidTimestamp, "36.00"},
		{"2024", "1", testStationORD, testValidTimestamp, "33.00"},
	}

	got, err := dedupWeatherRows(columns, rows)
	if err != nil {
		t.Fatalf("dedupWeatherRows() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	if got[0][2] != testStationJFK {
		t.Errorf("first remaining station = %q, want JFK", got[0][2])
	}

	if got[1][2] != testStationORD || got[1][4] != "33.00" {
		t.Errorf("ORD row = %v, want last-wins tmpf 33.00", got[1])
	}
}

func TestDedupWeatherRowsNoDuplicates(t *testing.T) {
	columns := []string{colStation, colValid}
	rows := [][]string{
		{testStationORD, testValidTimestamp},
		{testStationJFK, testValidTimestamp},
	}

	got, err := dedupWeatherRows(columns, rows)
	if err != nil {
		t.Fatalf("dedupWeatherRows() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
}

func TestServiceImportMonthDedup(t *testing.T) {
	ctx := context.Background()

	var gotRows [][]string

	st := &storetest.Stub{
		ReplaceWeatherObservationsByMonthFn: func(_ context.Context, _ int, _ int, _ []string, rows [][]string) error {
			gotRows = rows

			return nil
		},
	}

	opener := func(context.Context, int, int, []string) (string, func(), error) {
		file, err := os.CreateTemp(t.TempDir(), "asos-dup-*.csv")
		if err != nil {
			t.Fatalf("CreateTemp() error = %v", err)
		}

		const csvBody = "station,valid,tmpf,dwpf,relh,drct,sknt,gust,vsby,skyc1,skyc2,skyc3,skyl1,skyl2,skyl3,wxcodes,p01i,alti,mslp,metar\n" +
			"ORD,2024-01-01 00:51,32.00,28.00,84.98,310.00,11.00,,10.00,OVC,,,1500.00,,,-SN,0.0001,30.05,1018.20,KORD first\n" +
			"ORD,2024-01-01 00:51,33.00,28.00,84.98,310.00,11.00,,10.00,OVC,,,1500.00,,,-SN,0.0001,30.05,1018.20,KORD second\n"

		if _, err := file.WriteString(csvBody); err != nil {
			t.Fatalf("WriteString() error = %v", err)
		}

		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		return file.Name(), func() {}, nil
	}

	svc := NewService(st, nil).WithCSVOpener(opener)

	result, err := svc.ImportMonth(ctx, 2024, 1, []string{testStationORD})
	if err != nil {
		t.Fatalf("ImportMonth() error = %v", err)
	}

	if result.RowsImported != 1 {
		t.Fatalf("RowsImported = %d, want 1", result.RowsImported)
	}

	if len(gotRows) != 1 {
		t.Fatalf("len(gotRows) = %d, want 1", len(gotRows))
	}

	metarIdx := -1

	for i, col := range DBColumns {
		if col == colMetar {
			metarIdx = i

			break
		}
	}

	if metarIdx < 0 {
		t.Fatal("metar column missing")
	}

	if gotRows[0][metarIdx] != "KORD second" {
		t.Errorf("metar = %q, want last-wins KORD second", gotRows[0][metarIdx])
	}
}

func TestParseIEMValidUTC(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "2024-01-01 00:51", want: testValidTimestamp},
		{raw: "2024-01-01 00:51:30", want: "2024-01-01T00:51:30Z"},
		{raw: testValidTimestamp, want: testValidTimestamp},
	}

	for _, tt := range tests {
		got, err := parseIEMValidUTC(tt.raw)
		if err != nil {
			t.Fatalf("parseIEMValidUTC(%q) error = %v", tt.raw, err)
		}

		if got.Format(time.RFC3339) != tt.want {
			t.Errorf("parseIEMValidUTC(%q) = %s, want %s", tt.raw, got.Format(time.RFC3339), tt.want)
		}
	}
}

func TestServiceImportStations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	var (
		stationRows [][]string
		mappingRows [][]string
	)

	st := &storetest.Stub{
		DistinctFlightAirportCodesFn: func(context.Context) ([]string, error) {
			return []string{testStationORD, testStationXYZ}, nil
		},
		ReplaceWeatherStationsFn: func(_ context.Context, _ []string, stations [][]string, _ []string, mapping [][]string) error {
			stationRows = stations
			mappingRows = mapping

			return nil
		},
	}

	catalog := NewNetworkCatalog(server.URL, 0)
	catalog.networks = []string{testNetworkILASOS}
	svc := NewService(st, nil).WithCatalog(catalog)

	result, err := svc.ImportStations(context.Background())
	if err != nil {
		t.Fatalf("ImportStations() error = %v", err)
	}

	if result.StationsLoaded != 1 || result.MatchedAirports != 1 || result.UnmatchedAirports != 1 {
		t.Fatalf("result = %+v, want 1 station, 1 matched, 1 unmatched", result)
	}

	if len(result.Unmatched) != 1 || result.Unmatched[0] != testStationXYZ {
		t.Fatalf("unmatched = %v, want [XYZ]", result.Unmatched)
	}

	if len(stationRows) != 1 || stationRows[0][0] != testStationORD || stationRows[0][3] != testTzChicago {
		t.Fatalf("stationRows = %v", stationRows)
	}

	if len(mappingRows) != 2 {
		t.Fatalf("mappingRows = %v, want 2", mappingRows)
	}

	if mappingRows[0][0] != testStationORD || mappingRows[0][3] != "true" {
		t.Fatalf("matched row = %v", mappingRows[0])
	}

	if mappingRows[1][0] != testStationXYZ || mappingRows[1][3] != "false" || mappingRows[1][1] != "" {
		t.Fatalf("unmatched row = %v", mappingRows[1])
	}
}

func TestServiceImportStationsEmptyBTS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testGeoJSONFeatures: []map[string]any{
				{testGeoJSONProperties: map[string]any{testGeoJSONSID: testStationORD}},
			},
		})
	}))
	defer server.Close()

	var mappingRows [][]string

	st := &storetest.Stub{
		DistinctFlightAirportCodesFn: func(context.Context) ([]string, error) {
			return nil, nil
		},
		ReplaceWeatherStationsFn: func(_ context.Context, _ []string, _ [][]string, _ []string, mapping [][]string) error {
			mappingRows = mapping

			return nil
		},
	}

	catalog := NewNetworkCatalog(server.URL, 0)
	catalog.networks = []string{testNetworkILASOS}
	svc := NewService(st, nil).WithCatalog(catalog)

	result, err := svc.ImportStations(context.Background())
	if err != nil {
		t.Fatalf("ImportStations() error = %v", err)
	}

	if result.StationsLoaded != 1 || result.MatchedAirports != 0 || result.UnmatchedAirports != 0 {
		t.Fatalf("result = %+v", result)
	}

	if len(mappingRows) != 0 {
		t.Fatalf("mappingRows = %v, want empty", mappingRows)
	}
}
