package bts

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func TestServiceImportMonth(t *testing.T) {
	ctx := context.Background()
	s := mem.New()
	svc := NewService(s, nil).WithCSVOpener(minimalCSVOpener(t))

	result, err := svc.ImportMonth(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("ImportMonth() error = %v", err)
	}

	if result.RowsImported != TestdataRowCount {
		t.Fatalf("RowsImported = %d, want %d", result.RowsImported, TestdataRowCount)
	}

	if result.Year != 2026 || result.Month != 4 {
		t.Errorf("result = %+v, want year 2026 month 4", result)
	}

	withData, err := s.MonthsWithOnTimeFlightData(ctx, []model.YearMonth{{Year: 2026, Month: 4}})
	if err != nil {
		t.Fatalf("MonthsWithOnTimeFlightData() error = %v", err)
	}

	if len(withData) != 1 {
		t.Fatalf("len(withData) = %d, want 1", len(withData))
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

func TestServiceImportMonthReplace(t *testing.T) {
	ctx := context.Background()
	s := mem.New()
	svc := NewService(s, nil).WithCSVOpener(minimalCSVOpener(t))

	if _, err := svc.ImportMonth(ctx, 2026, 4); err != nil {
		t.Fatalf("first ImportMonth() error = %v", err)
	}

	first, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    "JFK",
		Dest:      "FLL",
		StartDate: "2026-04-01",
		EndDate:   "2026-04-30",
	})
	if err != nil {
		t.Fatalf("RouteStats() error = %v", err)
	}

	if first.Flights == 0 {
		t.Fatal("expected flights after first import")
	}

	if _, err := svc.ImportMonth(ctx, 2026, 4); err != nil {
		t.Fatalf("second ImportMonth() error = %v", err)
	}

	second, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    "JFK",
		Dest:      "FLL",
		StartDate: "2026-04-01",
		EndDate:   "2026-04-30",
	})
	if err != nil {
		t.Fatalf("RouteStats() after replace error = %v", err)
	}

	if second.Flights == 0 {
		t.Fatal("expected flights after replace import")
	}
}

func TestServiceImportMonthWithoutDownloader(t *testing.T) {
	svc := NewService(mem.New(), nil)

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

func TestTestdataCSVResolvable(t *testing.T) {
	path, err := TestdataCSV()
	if err != nil {
		t.Fatalf("TestdataCSV() error = %v", err)
	}

	if filepath.Base(path) != "on_time_2026_04.csv" {
		t.Errorf("basename = %q, want on_time_2026_04.csv", filepath.Base(path))
	}
}
