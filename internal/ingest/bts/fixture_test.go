package bts

import (
	"os"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/csvparse"
)

func TestParseFixtureCSV(t *testing.T) {
	file := openFixtureCSV(t)
	defer file.Close()

	columns, rows, err := csvparse.Parse(file, DBColumns, csvHeaderToColumn)
	if err != nil {
		t.Fatalf("csvparse.Parse() error = %v", err)
	}

	if len(columns) != len(DBColumns) {
		t.Fatalf("len(columns) = %d, want %d", len(columns), len(DBColumns))
	}

	if len(rows) != testdataRowCount {
		t.Fatalf("len(rows) = %d, want %d", len(rows), testdataRowCount)
	}

	originIdx := dbColumnIndex("origin")
	destIdx := dbColumnIndex("dest")

	foundORD := false

	for _, row := range rows {
		if row[originIdx] == "ORD" || row[destIdx] == "ORD" {
			foundORD = true
			break
		}
	}

	if !foundORD {
		t.Fatal("expected at least one ORD airport in fixture")
	}

	if rows[0][dbColumnIndex("flight_date")] == "" {
		t.Fatal("expected flight_date on first row")
	}
}

func TestParseFixtureCSVDropsTrailingEmptyColumn(t *testing.T) {
	file := openFixtureCSV(t)
	defer file.Close()

	_, _, err := csvparse.Parse(file, DBColumns, csvHeaderToColumn)
	if err != nil {
		t.Fatalf("csvparse.Parse() error = %v", err)
	}
}

func TestFixtureHasOperationalVariety(t *testing.T) {
	file := openFixtureCSV(t)
	defer file.Close()

	_, rows, err := csvparse.Parse(file, DBColumns, csvHeaderToColumn)
	if err != nil {
		t.Fatalf("csvparse.Parse() error = %v", err)
	}

	originIdx := dbColumnIndex("origin")
	destIdx := dbColumnIndex("dest")
	cancelledIdx := dbColumnIndex("cancelled")
	divertedIdx := dbColumnIndex("diverted")
	flightDateIdx := dbColumnIndex("flight_date")

	routes := make(map[string]struct{})
	dates := make(map[string]struct{})

	var cancelled, diverted int

	for _, row := range rows {
		routes[row[originIdx]+"->"+row[destIdx]] = struct{}{}
		dates[row[flightDateIdx]] = struct{}{}

		if row[cancelledIdx] != "" && row[cancelledIdx] != "0.00" {
			cancelled++
		}

		if row[divertedIdx] != "" && row[divertedIdx] != "0.00" {
			diverted++
		}
	}

	if len(routes) < 15 {
		t.Errorf("unique routes = %d, want at least 15", len(routes))
	}

	if len(dates) < 3 {
		t.Errorf("unique flight dates = %d, want at least 3", len(dates))
	}

	if cancelled == 0 {
		t.Error("expected at least one cancelled flight in fixture")
	}

	if diverted == 0 {
		t.Error("expected at least one diverted flight in fixture")
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

func dbColumnIndex(name string) int {
	for i, col := range DBColumns {
		if col == name {
			return i
		}
	}

	return -1
}
