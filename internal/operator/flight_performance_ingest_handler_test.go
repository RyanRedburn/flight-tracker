package operator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func btsFixtureCSVPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "..", "ingest", "bts", "testdata", "on_time_2026_04.csv")

	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("bts testdata csv missing at %s: %v", path, err)
	}

	return path
}

func TestFlightPerformanceIngestHandlerProcess(t *testing.T) {
	ctx := context.Background()

	var replaceYear, replaceMonth int

	var replaceRows int

	st := &storetest.Stub{
		GetFlightPerformanceIngestJobFn: func(_ context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
		ReplaceFlightPerformanceByMonthFn: func(_ context.Context, year, month int, _ []string, rows [][]string) error {
			replaceYear, replaceMonth = year, month
			replaceRows = len(rows)

			return nil
		},
		RebuildRouteTravelWindowsFn: func(context.Context) error {
			return nil
		},
		RebuildRouteWeatherStatsFn: func(context.Context) error {
			return nil
		},
	}

	path := btsFixtureCSVPath(t)
	svc := bts.NewService(st, nil).WithCSVOpener(func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	})

	h := NewFlightPerformanceIngestHandler(st, svc)
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	payload, err := h.Process(ctx, job)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if replaceYear != 2026 || replaceMonth != 4 {
		t.Errorf("ReplaceFlightPerformanceByMonth = %d-%d, want 2026-4", replaceYear, replaceMonth)
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if result["rows_imported"] == nil || result["rows_imported"].(float64) != float64(replaceRows) {
		t.Fatalf("result = %v, want rows_imported = %d", result, replaceRows)
	}
}

func TestFlightPerformanceIngestHandlerRebuildsTravelWindows(t *testing.T) {
	ctx := context.Background()

	rebuilds := 0
	st := flightPerformanceIngestStub(t, func(context.Context) error {
		rebuilds++

		return nil
	})

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if _, err := h.Process(ctx, job); err != nil {
		t.Fatalf("second Process() error = %v", err)
	}

	if rebuilds != 2 {
		t.Fatalf("rebuilds = %d, want 2 (idempotent full replace per ingest)", rebuilds)
	}
}

func TestFlightPerformanceIngestHandlerRebuildError(t *testing.T) {
	ctx := context.Background()
	st := flightPerformanceIngestStub(t, func(context.Context) error {
		return errRebuildFailed
	})

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err == nil {
		t.Fatal("expected rebuild error")
	}
}

func TestFlightPerformanceIngestHandlerSkipsRebuildOnImportError(t *testing.T) {
	ctx := context.Background()
	st := &storetest.Stub{
		GetFlightPerformanceIngestJobFn: func(_ context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
	}

	svc := bts.NewService(st, nil).WithCSVOpener(func(context.Context, int, int) (string, func(), error) {
		return "", func() {}, errImportFailed
	})

	h := NewFlightPerformanceIngestHandler(st, svc)
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err == nil {
		t.Fatal("expected import error")
	}
}

func flightPerformanceIngestStub(t *testing.T, rebuild func(context.Context) error) *storetest.Stub {
	t.Helper()

	return &storetest.Stub{
		GetFlightPerformanceIngestJobFn: func(_ context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
		ReplaceFlightPerformanceByMonthFn: func(context.Context, int, int, []string, [][]string) error {
			return nil
		},
		RebuildRouteTravelWindowsFn: rebuild,
		RebuildRouteWeatherStatsFn: func(context.Context) error {
			return nil
		},
	}
}

func TestFlightPerformanceIngestHandlerRebuildsWeatherStats(t *testing.T) {
	ctx := context.Background()

	var order []string

	st := &storetest.Stub{
		GetFlightPerformanceIngestJobFn: func(_ context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
		ReplaceFlightPerformanceByMonthFn: func(context.Context, int, int, []string, [][]string) error {
			return nil
		},
		RebuildRouteTravelWindowsFn: func(context.Context) error {
			order = append(order, "travel")

			return nil
		},
		RebuildRouteWeatherStatsFn: func(context.Context) error {
			order = append(order, "weather")

			return nil
		},
	}

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(order) != 2 || order[0] != "travel" || order[1] != "weather" {
		t.Fatalf("rebuild order = %v, want travel then weather", order)
	}
}

func TestFlightPerformanceIngestHandlerWeatherRebuildError(t *testing.T) {
	ctx := context.Background()
	st := flightPerformanceIngestStub(t, func(context.Context) error {
		return nil
	})
	st.RebuildRouteWeatherStatsFn = func(context.Context) error {
		return errRebuildFailed
	}

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err == nil {
		t.Fatal("expected weather rebuild error")
	}
}

func btsFixtureService(t *testing.T, st *storetest.Stub) *bts.Service {
	t.Helper()

	path := btsFixtureCSVPath(t)

	return bts.NewService(st, nil).WithCSVOpener(func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	})
}
