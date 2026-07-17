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

func TestCreateReferenceIngestJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	job, err := s.CreateReferenceIngestJob(ctx, model.JobTypeImportCountries)
	if err != nil {
		t.Fatalf("CreateReferenceIngestJob() error = %v", err)
	}

	if job.Type != model.JobTypeImportCountries {
		t.Errorf("Type = %q, want %q", job.Type, model.JobTypeImportCountries)
	}

	if job.Status != model.JobStatusPending {
		t.Errorf("Status = %q, want pending", job.Status)
	}
}

func TestCreateReferenceIngestJobInvalidType(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	_, err := s.CreateReferenceIngestJob(ctx, "invalid_job_type")
	if err == nil {
		t.Fatal("expected error for invalid job type")
	}
}

func TestHasReferenceData(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	hasData, err := s.HasReferenceData(ctx, store.ReferenceCountries)
	if err != nil {
		t.Fatalf("HasReferenceData() error = %v", err)
	}

	if hasData {
		t.Fatal("expected no countries data before import")
	}

	columns := testCountryColumns()
	rows := [][]string{testCountryRow("302672", "AD", "Andorra")}

	if err := s.ReplaceCountries(ctx, columns, rows); err != nil {
		t.Fatalf("ReplaceCountries() error = %v", err)
	}

	hasData, err = s.HasReferenceData(ctx, store.ReferenceCountries)
	if err != nil {
		t.Fatalf("HasReferenceData() error = %v", err)
	}

	if !hasData {
		t.Fatal("expected countries data after import")
	}
}

func TestReplaceCountriesReplacesAllRows(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	columns := testCountryColumns()

	if err := s.ReplaceCountries(ctx, columns, [][]string{
		testCountryRow("1", "AA", "Alpha"),
	}); err != nil {
		t.Fatalf("first ReplaceCountries() error = %v", err)
	}

	if err := s.ReplaceCountries(ctx, columns, [][]string{
		testCountryRow("2", "BB", "Beta"),
	}); err != nil {
		t.Fatalf("second ReplaceCountries() error = %v", err)
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

func TestReplaceCountriesStoresEmptyAsNull(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	columns := testCountryColumns()
	rows := [][]string{testCountryRow("302672", "AD", "Andorra")}

	if err := s.ReplaceCountries(ctx, columns, rows); err != nil {
		t.Fatalf("ReplaceCountries() error = %v", err)
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

func TestHasReferenceDataInvalidDataset(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	_, err := s.HasReferenceData(ctx, store.ReferenceDataset("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid dataset")
	}
}
