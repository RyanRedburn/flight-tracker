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

	if len(rows) == 0 {
		t.Fatal("expected at least one data row")
	}

	if rows[0][5] != "2026-04-24" {
		t.Errorf("flight_date = %q, want 2026-04-24", rows[0][5])
	}

	if rows[0][23] != "ORD" {
		t.Errorf("origin = %q, want ORD", rows[0][23])
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
