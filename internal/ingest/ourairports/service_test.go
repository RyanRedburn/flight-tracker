package ourairports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func TestDownloaderDownloadsCSV(t *testing.T) {
	const body = "id,code,name,continent,wikipedia_link,keywords\n1,AA,Alpha,EU,,\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)

	path, cleanup, err := d.DownloadCSV(context.Background(), store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("DownloadCSV() error = %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != body {
		t.Errorf("file content = %q, want %q", string(data), body)
	}
}

func TestServiceImportCountries(t *testing.T) {
	ctx := context.Background()
	s := mem.New()
	svc := NewService(s, nil).WithCSVOpener(fixtureCSVOpener(t))

	result, err := svc.Import(ctx, store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.RowsImported != testdataRowCount {
		t.Fatalf("RowsImported = %d, want %d", result.RowsImported, testdataRowCount)
	}

	if result.Dataset != store.OurAirportsCountries {
		t.Errorf("Dataset = %q, want %q", result.Dataset, store.OurAirportsCountries)
	}

	hasData, err := s.HasOurAirportsData(ctx, store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if !hasData {
		t.Fatal("expected countries data after import")
	}
}

func TestServiceImportReplace(t *testing.T) {
	ctx := context.Background()
	s := mem.New()
	svc := NewService(s, nil).WithCSVOpener(fixtureCSVOpener(t))

	if _, err := svc.Import(ctx, store.OurAirportsCountries); err != nil {
		t.Fatalf("first Import() error = %v", err)
	}

	if _, err := svc.Import(ctx, store.OurAirportsCountries); err != nil {
		t.Fatalf("second Import() error = %v", err)
	}

	hasData, err := s.HasOurAirportsData(ctx, store.OurAirportsCountries)
	if err != nil {
		t.Fatalf("HasOurAirportsData() error = %v", err)
	}

	if !hasData {
		t.Fatal("expected countries data after replace import")
	}
}

func TestServiceImportWithoutDownloader(t *testing.T) {
	svc := NewService(mem.New(), nil)

	_, err := svc.Import(context.Background(), store.OurAirportsCountries)
	if err == nil {
		t.Fatal("Import() expected error without downloader or opener")
	}
}

func TestImportResultJSON(t *testing.T) {
	payload, err := ImportResult{
		Dataset:      store.OurAirportsAirports,
		RowsImported: 42,
	}.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded[jsonKeyDataset] != string(store.OurAirportsAirports) {
		t.Errorf("dataset = %v, want airports", decoded[jsonKeyDataset])
	}

	if decoded[jsonKeyRows] != float64(42) {
		t.Errorf("rows_imported = %v, want 42", decoded[jsonKeyRows])
	}
}

func TestDatasetHelpers(t *testing.T) {
	jobType, err := JobType(store.OurAirportsRegions)
	if err != nil {
		t.Fatalf("JobType() error = %v", err)
	}

	if jobType == "" {
		t.Fatal("expected job type")
	}

	filename, err := CSVFilename(store.OurAirportsRegions)
	if err != nil {
		t.Fatalf("CSVFilename() error = %v", err)
	}

	if filename != "regions.csv" {
		t.Errorf("filename = %q, want regions.csv", filename)
	}
}
