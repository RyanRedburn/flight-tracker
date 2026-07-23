package operator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func iemFixtureCSVPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "..", "ingest", "iem", "testdata", "asos_2024_01.csv")

	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("iem testdata csv missing at %s: %v", path, err)
	}

	return path
}

func TestWeatherIngestHandlerProcess(t *testing.T) {
	ctx := context.Background()

	var replaceYear, replaceMonth int

	var replaceRows int

	st := &storetest.Stub{
		GetWeatherIngestJobFn: func(_ context.Context, jobID string) (*model.WeatherIngestJob, error) {
			return &model.WeatherIngestJob{
				JobID:    jobID,
				Year:     2024,
				Month:    1,
				Stations: []string{"ORD", "JFK"},
			}, nil
		},
		ReplaceWeatherObservationsByMonthFn: func(_ context.Context, year, month int, _ []string, rows [][]string) error {
			replaceYear, replaceMonth = year, month
			replaceRows = len(rows)

			return nil
		},
	}

	path := iemFixtureCSVPath(t)
	svc := iem.NewService(st, nil).WithCSVOpener(func(context.Context, int, int, []string) (string, func(), error) {
		return path, func() {}, nil
	})

	h := NewWeatherIngestHandler(st, svc)
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportWeatherObservations}

	payload, err := h.Process(ctx, job)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if replaceYear != 2024 || replaceMonth != 1 {
		t.Errorf("ReplaceWeatherObservationsByMonth = %d-%d, want 2024-1", replaceYear, replaceMonth)
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if result["rows_imported"] == nil || result["rows_imported"].(float64) != float64(replaceRows) {
		t.Fatalf("result = %v, want rows_imported = %d", result, replaceRows)
	}
}
