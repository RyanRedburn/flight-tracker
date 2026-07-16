package mem

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const testColKeywords = "keywords"

func testCountryColumns() []string {
	return []string{"id", "code", "name", "continent", "wikipedia_link", testColKeywords}
}

func testCountryRow(id, code, name string) []string {
	return []string{id, code, name, "EU", "", testColKeywords}
}

func TestMemCreateOurAirportsIngestJob(t *testing.T) {
	ctx := context.Background()
	s := New()

	job, err := s.CreateOurAirportsIngestJob(ctx, model.JobTypeImportOurAirportsRegions)
	if err != nil {
		t.Fatalf("CreateOurAirportsIngestJob() error = %v", err)
	}

	if job.Type != model.JobTypeImportOurAirportsRegions {
		t.Errorf("Type = %q, want %q", job.Type, model.JobTypeImportOurAirportsRegions)
	}
}

func TestMemHasOurAirportsData(t *testing.T) {
	ctx := context.Background()
	s := New()

	hasData, err := s.HasOurAirportsData(ctx, store.OurAirportsAirports)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if hasData {
		t.Fatal("expected no airport data before import")
	}

	columns := []string{"id", "ident", "type", "name", "latitude_deg", "longitude_deg", "elevation_ft", "continent", "iso_country", "iso_region", "municipality", "scheduled_service", "icao_code", "iata_code", "gps_code", "local_code", "home_link", "wikipedia_link", "keywords"}
	rows := [][]string{{
		"6523", "00A", "heliport", "Total RF Heliport", "40.070985", "-74.933689", "11",
		"NA", "US", "US-PA", "Bensalem", "no", "", "", "K00A", "00A", "", "", "",
	}}

	if err := s.ReplaceOurAirportsAirports(ctx, columns, rows); err != nil {
		t.Fatalf("ReplaceOurAirportsAirports() error = %v", err)
	}

	hasData, err = s.HasOurAirportsData(ctx, store.OurAirportsAirports)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if !hasData {
		t.Fatal("expected airport data after import")
	}
}

func TestMemReplaceOurAirportsCountriesReplacesAllRows(t *testing.T) {
	ctx := context.Background()
	s := New()

	columns := testCountryColumns()

	if err := s.ReplaceOurAirportsCountries(ctx, columns, [][]string{
		testCountryRow("1", "AA", "Alpha"),
	}); err != nil {
		t.Fatalf("first ReplaceOurAirportsCountries() error = %v", err)
	}

	if err := s.ReplaceOurAirportsCountries(ctx, columns, [][]string{
		testCountryRow("2", "BB", "Beta"),
	}); err != nil {
		t.Fatalf("second ReplaceOurAirportsCountries() error = %v", err)
	}

	hasData, err := s.HasOurAirportsData(ctx, store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if !hasData {
		t.Fatal("expected countries data after replace")
	}

	s.mu.Lock()
	rows, err := s.ourAirportsRowsLocked(store.OurAirportsCountries)
	s.mu.Unlock()

	if err != nil {
		t.Fatalf("ourAirportsRowsLocked() error = %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}

	if rows[0][1] != "BB" {
		t.Errorf("code = %q, want BB", rows[0][1])
	}
}
