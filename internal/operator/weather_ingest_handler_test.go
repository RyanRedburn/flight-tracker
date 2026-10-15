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
	"github.com/RyanRedburn/flight-tracker/internal/store"
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
				Stations: []string{testStationORD, "JFK"},
			}, nil
		},
		ReplaceWeatherObservationsByMonthFn: func(_ context.Context, year, month int, _ []string, rows [][]string) error {
			replaceYear, replaceMonth = year, month
			replaceRows = len(rows)

			return nil
		},
		CreateRebuildRouteWeatherStatsJobFn: func(context.Context) (*model.Job, error) {
			return queuedRebuild(model.JobTypeRebuildRouteWeatherStats), nil
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

func TestWeatherIngestHandlerEnqueuesWeatherStats(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		create  func(context.Context) (*model.Job, error)
		wantErr bool
	}{
		{
			name: "queued",
			create: func(context.Context) (*model.Job, error) {
				return queuedRebuild(model.JobTypeRebuildRouteWeatherStats), nil
			},
		},
		{
			name: "pending",
			create: func(context.Context) (*model.Job, error) {
				return nil, store.ErrActiveIngestConflict
			},
		},
		{
			name: "error",
			create: func(context.Context) (*model.Job, error) {
				return nil, errEnqueueFailed
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			st := &storetest.Stub{
				GetWeatherIngestJobFn: func(_ context.Context, jobID string) (*model.WeatherIngestJob, error) {
					return &model.WeatherIngestJob{JobID: jobID, Year: 2024, Month: 1, Stations: []string{testStationORD}}, nil
				},
				ReplaceWeatherObservationsByMonthFn: func(context.Context, int, int, []string, [][]string) error {
					return nil
				},
				CreateRebuildRouteWeatherStatsJobFn: func(ctx context.Context) (*model.Job, error) {
					calls++

					return tt.create(ctx)
				},
			}

			path := iemFixtureCSVPath(t)
			svc := iem.NewService(st, nil).WithCSVOpener(func(context.Context, int, int, []string) (string, func(), error) {
				return path, func() {}, nil
			})

			h := NewWeatherIngestHandler(st, svc)
			_, err := h.Process(ctx, &model.Job{ID: testJobID})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected enqueue error")
				}
			} else if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			if calls != 1 {
				t.Fatalf("enqueue calls = %d, want 1", calls)
			}
		})
	}
}

func TestWeatherIngestHandlerSkipsRebuildOnImportError(t *testing.T) {
	ctx := context.Background()
	st := &storetest.Stub{
		GetWeatherIngestJobFn: func(_ context.Context, jobID string) (*model.WeatherIngestJob, error) {
			return &model.WeatherIngestJob{JobID: jobID, Year: 2024, Month: 1, Stations: []string{testStationORD}}, nil
		},
	}

	svc := iem.NewService(st, nil).WithCSVOpener(func(context.Context, int, int, []string) (string, func(), error) {
		return "", func() {}, errImportFailed
	})

	h := NewWeatherIngestHandler(st, svc)
	if _, err := h.Process(ctx, &model.Job{ID: testJobID}); err == nil {
		t.Fatal("expected import error")
	}
}
