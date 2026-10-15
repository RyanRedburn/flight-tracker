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
	"github.com/RyanRedburn/flight-tracker/internal/store"
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

	var queued []model.JobType

	st := &storetest.Stub{
		GetFlightPerformanceIngestJobFn: func(_ context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
		ReplaceFlightPerformanceByMonthFn: func(_ context.Context, year, month int, _ []string, rows [][]string) error {
			replaceYear, replaceMonth = year, month
			replaceRows = len(rows)

			return nil
		},
		CreateRebuildRouteTravelWindowsJobFn: func(context.Context) (*model.Job, error) {
			queued = append(queued, model.JobTypeRebuildRouteTravelWindows)

			return queuedRebuild(model.JobTypeRebuildRouteTravelWindows), nil
		},
		CreateRebuildRouteWeatherStatsJobFn: func(context.Context) (*model.Job, error) {
			queued = append(queued, model.JobTypeRebuildRouteWeatherStats)

			return queuedRebuild(model.JobTypeRebuildRouteWeatherStats), nil
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

	if len(queued) != 2 || queued[0] != model.JobTypeRebuildRouteTravelWindows || queued[1] != model.JobTypeRebuildRouteWeatherStats {
		t.Fatalf("queued = %v, want travel windows then weather stats", queued)
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if result["rows_imported"] == nil || result["rows_imported"].(float64) != float64(replaceRows) {
		t.Fatalf("result = %v, want rows_imported = %d", result, replaceRows)
	}
}

func TestFlightPerformanceIngestHandlerSkipsPendingRebuild(t *testing.T) {
	ctx := context.Background()

	queued := 0
	st := flightPerformanceIngestStub(t, func(context.Context) (*model.Job, error) {
		queued++

		return nil, store.ErrActiveIngestConflict
	}, func(context.Context) (*model.Job, error) {
		queued++

		return nil, store.ErrActiveIngestConflict
	})

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if queued != 2 {
		t.Fatalf("enqueue attempts = %d, want 2", queued)
	}
}

func TestFlightPerformanceIngestHandlerEnqueueError(t *testing.T) {
	ctx := context.Background()

	weatherQueued := false
	st := flightPerformanceIngestStub(t, func(context.Context) (*model.Job, error) {
		return nil, errEnqueueFailed
	}, func(context.Context) (*model.Job, error) {
		weatherQueued = true

		return queuedRebuild(model.JobTypeRebuildRouteWeatherStats), nil
	})

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err == nil {
		t.Fatal("expected enqueue error")
	}

	if weatherQueued {
		t.Fatal("weather rebuild queued after travel-window enqueue failed")
	}
}

func TestFlightPerformanceIngestHandlerSkipsEnqueueOnImportError(t *testing.T) {
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

func flightPerformanceIngestStub(
	t *testing.T,
	travel func(context.Context) (*model.Job, error),
	weather func(context.Context) (*model.Job, error),
) *storetest.Stub {
	t.Helper()

	return &storetest.Stub{
		GetFlightPerformanceIngestJobFn: func(_ context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
		ReplaceFlightPerformanceByMonthFn: func(context.Context, int, int, []string, [][]string) error {
			return nil
		},
		CreateRebuildRouteTravelWindowsJobFn: travel,
		CreateRebuildRouteWeatherStatsJobFn:  weather,
	}
}

func TestFlightPerformanceIngestHandlerWeatherEnqueueError(t *testing.T) {
	ctx := context.Background()
	st := flightPerformanceIngestStub(t, func(context.Context) (*model.Job, error) {
		return queuedRebuild(model.JobTypeRebuildRouteTravelWindows), nil
	}, func(context.Context) (*model.Job, error) {
		return nil, errEnqueueFailed
	})

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	job := &model.Job{ID: testJobID, Type: model.JobTypeImportFlightPerformance}

	if _, err := h.Process(ctx, job); err == nil {
		t.Fatal("expected weather enqueue error")
	}
}

func TestFlightPerformanceIngestHandlerPendingTravelStillQueuesWeather(t *testing.T) {
	ctx := context.Background()

	var queued []model.JobType

	st := flightPerformanceIngestStub(t, func(context.Context) (*model.Job, error) {
		queued = append(queued, model.JobTypeRebuildRouteTravelWindows)

		return nil, store.ErrActiveIngestConflict
	}, func(context.Context) (*model.Job, error) {
		queued = append(queued, model.JobTypeRebuildRouteWeatherStats)

		return queuedRebuild(model.JobTypeRebuildRouteWeatherStats), nil
	})

	h := NewFlightPerformanceIngestHandler(st, btsFixtureService(t, st))
	if _, err := h.Process(ctx, &model.Job{ID: testJobID}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(queued) != 2 || queued[1] != model.JobTypeRebuildRouteWeatherStats {
		t.Fatalf("queued = %v, want weather stats after a pending travel-window skip", queued)
	}
}

func btsFixtureService(t *testing.T, st *storetest.Stub) *bts.Service {
	t.Helper()

	path := btsFixtureCSVPath(t)

	return bts.NewService(st, nil).WithCSVOpener(func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	})
}

func queuedRebuild(jobType model.JobType) *model.Job {
	return &model.Job{ID: "rebuild-" + string(jobType), Type: jobType, Status: model.JobStatusPending}
}
