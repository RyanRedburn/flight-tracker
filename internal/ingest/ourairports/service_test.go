package ourairports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
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

	path, cleanup, err := d.DownloadCSV(context.Background(), model.ReferenceCountries)
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

	datasets := []model.ReferenceDataset{
		model.ReferenceCountries,
		model.ReferenceRegions,
		model.ReferenceAirports,
	}

	for _, dataset := range datasets {
		t.Run(string(dataset), func(t *testing.T) {
			var replaced model.ReferenceDataset

			var gotRows int

			st := &storetest.Stub{
				ReplaceCountriesFn: func(_ context.Context, _ []string, rows [][]string) error {
					replaced = model.ReferenceCountries
					gotRows = len(rows)

					return nil
				},
				ReplaceRegionsFn: func(_ context.Context, _ []string, rows [][]string) error {
					replaced = model.ReferenceRegions
					gotRows = len(rows)

					return nil
				},
				ReplaceAirportsFn: func(_ context.Context, _ []string, rows [][]string) error {
					replaced = model.ReferenceAirports
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

	_, err := svc.Import(context.Background(), model.ReferenceCountries)
	if err == nil {
		t.Fatal("Import() expected error without downloader or opener")
	}
}
