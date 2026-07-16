//go:build cgo

package sqlite

import (
	"context"
	"database/sql"
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

func TestCreateOurAirportsIngestJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	job, err := s.CreateOurAirportsIngestJob(ctx, model.JobTypeImportOurAirportsCountries)
	if err != nil {
		t.Fatalf("CreateOurAirportsIngestJob() error = %v", err)
	}

	if job.Type != model.JobTypeImportOurAirportsCountries {
		t.Errorf("Type = %q, want %q", job.Type, model.JobTypeImportOurAirportsCountries)
	}

	if job.Status != model.JobStatusPending {
		t.Errorf("Status = %q, want pending", job.Status)
	}
}

func TestCreateOurAirportsIngestJobInvalidType(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	_, err := s.CreateOurAirportsIngestJob(ctx, "invalid_job_type")
	if err == nil {
		t.Fatal("expected error for invalid job type")
	}
}

func TestHasOurAirportsData(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	hasData, err := s.HasOurAirportsData(ctx, store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if hasData {
		t.Fatal("expected no countries data before import")
	}

	columns := testCountryColumns()
	rows := [][]string{testCountryRow("302672", "AD", "Andorra")}

	if err := s.ReplaceOurAirportsCountries(ctx, columns, rows); err != nil {
		t.Fatalf("ReplaceOurAirportsCountries() error = %v", err)
	}

	hasData, err = s.HasOurAirportsData(ctx, store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if !hasData {
		t.Fatal("expected countries data after import")
	}
}

func TestReplaceOurAirportsCountriesReplacesAllRows(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

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

	st, ok := s.(*Store)
	if !ok {
		t.Fatal("expected sqlite store")
	}

	var count int

	err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM countries`).Scan(&count)
	if err != nil {
		t.Fatalf("count countries: %v", err)
	}

	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	var code string

	err = st.db.QueryRowContext(ctx, `SELECT code FROM countries WHERE id = 2`).Scan(&code)
	if err != nil {
		t.Fatalf("select country: %v", err)
	}

	if code != "BB" {
		t.Errorf("code = %q, want BB", code)
	}
}

func TestReplaceOurAirportsCountriesStoresEmptyAsNull(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	columns := testCountryColumns()
	rows := [][]string{testCountryRow("302672", "AD", "Andorra")}

	if err := s.ReplaceOurAirportsCountries(ctx, columns, rows); err != nil {
		t.Fatalf("ReplaceOurAirportsCountries() error = %v", err)
	}

	st, ok := s.(*Store)
	if !ok {
		t.Fatal("expected sqlite store")
	}

	var link sql.NullString

	err := st.db.QueryRowContext(ctx, `SELECT wikipedia_link FROM countries WHERE id = 302672`).Scan(&link)
	if err != nil {
		t.Fatalf("select wikipedia_link: %v", err)
	}

	if link.Valid {
		t.Errorf("wikipedia_link = %q, want NULL", link.String)
	}
}

func TestHasOurAirportsDataInvalidDataset(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	_, err := s.HasOurAirportsData(ctx, store.OurAirportsDataset("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid dataset")
	}
}
