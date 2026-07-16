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
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
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

func TestServiceImportDatasets(t *testing.T) {
	ctx := context.Background()

	datasets := []store.OurAirportsDataset{
		store.OurAirportsCountries,
		store.OurAirportsRegions,
		store.OurAirportsAirports,
	}

	for _, dataset := range datasets {
		t.Run(string(dataset), func(t *testing.T) {
			var replaced store.OurAirportsDataset
			var gotRows int

			st := &storetest.Stub{
				ReplaceOurAirportsCountriesFn: func(_ context.Context, _ []string, rows [][]string) error {
					replaced = store.OurAirportsCountries
					gotRows = len(rows)

					return nil
				},
				ReplaceOurAirportsRegionsFn: func(_ context.Context, _ []string, rows [][]string) error {
					replaced = store.OurAirportsRegions
					gotRows = len(rows)

					return nil
				},
				ReplaceOurAirportsAirportsFn: func(_ context.Context, _ []string, rows [][]string) error {
					replaced = store.OurAirportsAirports
					gotRows = len(rows)

					return nil
				},
			}
			svc := NewService(st, nil).WithCSVOpener(fixtureCSVOpener(t))

			result, err := svc.Import(ctx, dataset)
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}

			if result.RowsImported != testdataRowCount {
				t.Fatalf("RowsImported = %d, want %d", result.RowsImported, testdataRowCount)
			}

			if result.Dataset != dataset {
				t.Errorf("Dataset = %q, want %q", result.Dataset, dataset)
			}

			if replaced != dataset || gotRows != testdataRowCount {
				t.Errorf("replace = %q rows=%d, want %q rows=%d", replaced, gotRows, dataset, testdataRowCount)
			}
		})
	}
}

func TestServiceImportWithoutDownloader(t *testing.T) {
	svc := NewService(&storetest.Stub{}, nil)

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
