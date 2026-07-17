package ourairports

import (
	"os"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestParseCountriesCSV(t *testing.T) {
	file, err := os.Open(testdataCSVPath(t, store.ReferenceCountries))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	columns, rows, err := ParseCSV(file, store.ReferenceCountries)
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}

	if len(columns) != len(countryColumns) {
		t.Fatalf("len(columns) = %d, want %d", len(columns), len(countryColumns))
	}

	if len(rows) != testdataRowCount {
		t.Fatalf("len(rows) = %d, want %d", len(rows), testdataRowCount)
	}

	if rows[0][1] != "AD" {
		t.Errorf("code = %q, want AD", rows[0][1])
	}
}

func TestParseRegionsCSV(t *testing.T) {
	file, err := os.Open(testdataCSVPath(t, store.ReferenceRegions))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	_, rows, err := ParseCSV(file, store.ReferenceRegions)
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}

	if len(rows) != testdataRowCount {
		t.Fatalf("len(rows) = %d, want %d", len(rows), testdataRowCount)
	}
}

func TestParseAirportsCSV(t *testing.T) {
	file, err := os.Open(testdataCSVPath(t, store.ReferenceAirports))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	_, rows, err := ParseCSV(file, store.ReferenceAirports)
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}

	if len(rows) != testdataRowCount {
		t.Fatalf("len(rows) = %d, want %d", len(rows), testdataRowCount)
	}

	if rows[0][1] != "00A" {
		t.Errorf("ident = %q, want 00A", rows[0][1])
	}
}

func TestParseCSVInvalidDataset(t *testing.T) {
	_, _, err := ParseCSV(nil, store.ReferenceDataset("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid dataset")
	}
}
