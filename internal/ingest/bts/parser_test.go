package bts

import (
	"os"
	"testing"
)

func TestDBColumnsCount(t *testing.T) {
	if len(DBColumns) != 119 {
		t.Fatalf("len(DBColumns) = %d, want 119", len(DBColumns))
	}
}

func TestCSVHeaderToColumnKnownFields(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{header: "Year", want: colYear},
		{header: "FlightDate", want: "flight_date"},
		{header: "DayofMonth", want: colDayOfMonth},
		{header: "Operating_Airline ", want: "operating_airline"},
		{header: "OriginAirportID", want: "origin_airport_id"},
		{header: "CRSDepTime", want: "crs_dep_time"},
		{header: "Duplicate", want: "duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := csvHeaderToColumn(tt.header); got != tt.want {
				t.Errorf("csvHeaderToColumn(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func dbColumnIndex(name string) int {
	for i, col := range DBColumns {
		if col == name {
			return i
		}
	}

	return -1
}

func TestParseCSVFromFixture(t *testing.T) {
	file, err := os.Open(testRepoCSVPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	columns, rows, err := ParseCSV(file)
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}

	if len(columns) != len(DBColumns) {
		t.Fatalf("len(columns) = %d, want %d", len(columns), len(DBColumns))
	}

	if len(rows) != TestdataRowCount {
		t.Fatalf("len(rows) = %d, want %d", len(rows), TestdataRowCount)
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

func TestParseCSVDropsTrailingEmptyColumn(t *testing.T) {
	file, err := os.Open(testRepoCSVPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	_, _, err = ParseCSV(file)
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
}

func TestFixtureHasOperationalVariety(t *testing.T) {
	file, err := os.Open(testRepoCSVPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	_, rows, err := ParseCSV(file)
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
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
