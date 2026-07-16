package bts

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestServiceImportMonth(t *testing.T) {
	ctx := context.Background()

	var gotYear, gotMonth int

	var gotRows int

	st := &storetest.Stub{
		ReplaceOnTimeFlightsByMonthFn: func(_ context.Context, year, month int, _ []string, rows [][]string) error {
			gotYear, gotMonth = year, month
			gotRows = len(rows)

			return nil
		},
	}
	svc := NewService(st, nil).WithCSVOpener(minimalCSVOpener(t))

	result, err := svc.ImportMonth(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("ImportMonth() error = %v", err)
	}

	if result.RowsImported != testdataRowCount {
		t.Fatalf("RowsImported = %d, want %d", result.RowsImported, testdataRowCount)
	}

	if result.Year != 2026 || result.Month != 4 {
		t.Errorf("result = %+v, want year 2026 month 4", result)
	}

	if gotYear != 2026 || gotMonth != 4 || gotRows != testdataRowCount {
		t.Errorf("ReplaceOnTimeFlightsByMonth = %d-%d rows=%d, want 2026-4 rows=%d",
			gotYear, gotMonth, gotRows, testdataRowCount)
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

	_, err := svc.ImportMonth(context.Background(), 2026, 4)
	if err == nil {
		t.Fatal("ImportMonth() expected error without downloader or opener")
	}
}

func TestImportResultJSON(t *testing.T) {
	payload, err := ImportResult{Year: 2026, Month: 4, RowsImported: 42}.MarshalJSON()
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
