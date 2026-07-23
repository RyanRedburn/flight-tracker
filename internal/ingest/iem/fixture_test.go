package iem

import (
	"os"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/csvparse"
)

func TestParseFixtureCSV(t *testing.T) {
	file := openFixtureCSV(t)
	defer file.Close()

	columns, rows, err := csvparse.Parse(file, ObservationColumns, csvHeaderToColumn)
	if err != nil {
		t.Fatalf("csvparse.Parse() error = %v", err)
	}

	if len(columns) != len(ObservationColumns) {
		t.Fatalf("len(columns) = %d, want %d", len(columns), len(ObservationColumns))
	}

	if len(rows) != testdataRowCount {
		t.Fatalf("len(rows) = %d, want %d", len(rows), testdataRowCount)
	}

	stationIdx := observationColumnIndex(colStation)
	if rows[0][stationIdx] != "ORD" {
		t.Errorf("first station = %q, want ORD", rows[0][stationIdx])
	}

	stations := make(map[string]struct{})
	for _, row := range rows {
		stations[row[stationIdx]] = struct{}{}
	}

	for _, want := range []string{"ORD", "JFK", "ATL", "DEN", "SFO"} {
		if _, ok := stations[want]; !ok {
			t.Errorf("expected station %s in fixture", want)
		}
	}

	// Empty cells (missing gust / temps) must remain empty for nullEmpty insert.
	gustIdx := observationColumnIndex("gust")
	if rows[0][gustIdx] != "" {
		t.Errorf("ORD first-row gust = %q, want empty", rows[0][gustIdx])
	}

	tmpfIdx := observationColumnIndex("tmpf")
	if rows[6][tmpfIdx] != "" {
		t.Errorf("ATL missing tmpf row = %q, want empty", rows[6][tmpfIdx])
	}
}

func TestDBColumnsIncludePartitionKeys(t *testing.T) {
	if len(DBColumns) != len(ObservationColumns)+2 {
		t.Fatalf("len(DBColumns) = %d, want %d", len(DBColumns), len(ObservationColumns)+2)
	}
	if DBColumns[0] != colYear || DBColumns[1] != colMonth {
		t.Fatalf("DBColumns prefix = %q,%q, want year,month", DBColumns[0], DBColumns[1])
	}
}

func openFixtureCSV(t *testing.T) *os.File {
	t.Helper()

	file, err := os.Open(fixtureCSVPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	return file
}

func observationColumnIndex(name string) int {
	for i, col := range ObservationColumns {
		if col == name {
			return i
		}
	}

	return -1
}
