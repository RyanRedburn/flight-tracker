package iem

import (
	"context"
	"encoding/json"
	"errors"
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
