package operator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

const oaTestdataRowCount = 2

func oaFixtureCSVPath(t *testing.T, dataset store.OurAirportsDataset) string {
	t.Helper()

	filename, err := ourairports.CSVFilename(dataset)
	if err != nil {
		t.Fatalf("CSVFilename() error = %v", err)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "..", "ingest", "ourairports", "testdata", filename)

	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ourairports testdata csv missing at %s: %v", path, err)
	}

	return path
}

func newTestOurAirportsIngestService(t *testing.T, st *mem.Store) *ourairports.Service {
	t.Helper()

	opener := func(_ context.Context, dataset store.OurAirportsDataset) (string, func(), error) {
		return oaFixtureCSVPath(t, dataset), func() {}, nil
	}

	return ourairports.NewService(st, nil).WithCSVOpener(opener)
}

func TestOurAirportsHandlerTypes(t *testing.T) {
	svc := newTestOurAirportsIngestService(t, mem.New())

	tests := []struct {
		name    string
		handler JobHandler
		want    string
	}{
		{"countries", NewCountriesHandler(svc), model.JobTypeImportOurAirportsCountries},
		{"regions", NewRegionsHandler(svc), model.JobTypeImportOurAirportsRegions},
		{"airports", NewAirportsHandler(svc), model.JobTypeImportOurAirportsAirports},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.handler.Type(); got != tt.want {
				t.Errorf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOurAirportsIngestHandlerImportsCSV(t *testing.T) {
	tests := []struct {
		name    string
		jobType string
		dataset store.OurAirportsDataset
		handler func(*ourairports.Service) JobHandler
	}{
		{
			name:    "countries",
			jobType: model.JobTypeImportOurAirportsCountries,
			dataset: store.OurAirportsCountries,
			handler: func(svc *ourairports.Service) JobHandler { return NewCountriesHandler(svc) },
		},
		{
			name:    "regions",
			jobType: model.JobTypeImportOurAirportsRegions,
			dataset: store.OurAirportsRegions,
			handler: func(svc *ourairports.Service) JobHandler { return NewRegionsHandler(svc) },
		},
		{
			name:    "airports",
			jobType: model.JobTypeImportOurAirportsAirports,
			dataset: store.OurAirportsAirports,
			handler: func(svc *ourairports.Service) JobHandler { return NewAirportsHandler(svc) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := mem.New()
			ctx := context.Background()
			svc := newTestOurAirportsIngestService(t, st)

			job, err := st.CreateOurAirportsIngestJob(ctx, tt.jobType)
			if err != nil {
				t.Fatalf("CreateOurAirportsIngestJob() error = %v", err)
			}

			claimed, err := st.ClaimNextPendingJob(ctx)
			if err != nil {
				t.Fatalf("ClaimNextPendingJob() error = %v", err)
			}

			if claimed.ID != job.ID {
				t.Fatalf("claimed ID = %q, want %q", claimed.ID, job.ID)
			}

			processor := NewProcessor(st, tt.handler(svc))
			if err := processor.Process(ctx, claimed); err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			got, err := st.GetJob(ctx, job.ID)
			if err != nil {
				t.Fatalf("GetJob() error = %v", err)
			}

			if got.Status != model.JobStatusCompleted {
				t.Errorf("Status = %q, want completed", got.Status)
			}

			var result map[string]any
			if err := json.Unmarshal(got.Result, &result); err != nil {
				t.Fatalf("Unmarshal result: %v", err)
			}

			if result["dataset"] != string(tt.dataset) {
				t.Errorf("dataset = %v, want %q", result["dataset"], tt.dataset)
			}

			if result["rows_imported"] == nil || result["rows_imported"].(float64) != float64(oaTestdataRowCount) {
				t.Fatalf("result = %v, want rows_imported = %d", result, oaTestdataRowCount)
			}

			hasData, err := st.HasOurAirportsData(ctx, tt.dataset)
			if err != nil {
				t.Fatalf("HasOurAirportsData() error = %v", err)
			}

			if !hasData {
				t.Fatal("expected ourairports data after import")
			}
		})
	}
}
