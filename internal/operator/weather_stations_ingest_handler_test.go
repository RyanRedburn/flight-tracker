package operator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestWeatherStationsHandlerType(t *testing.T) {
	h := NewWeatherStationsHandler(&storetest.Stub{}, iem.NewService(&storetest.Stub{}, nil))
	if got := h.Type(); got != model.JobTypeImportWeatherStations {
		t.Errorf("Type() = %q, want %q", got, model.JobTypeImportWeatherStations)
	}
}

func TestWeatherStationsHandlerSkipsRebuildOnImportError(t *testing.T) {
	h := NewWeatherStationsHandler(&storetest.Stub{}, iem.NewService(&storetest.Stub{}, nil))

	if _, err := h.Process(context.Background(), &model.Job{ID: testJobID}); err == nil {
		t.Fatal("expected import error")
	}
}

func TestWeatherStationsHandlerEnqueuesWeatherStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"features":[{"properties":{"sid":"ORD","network":"IL_ASOS","tzname":"America/Chicago"}}]}`)); err != nil {
			t.Errorf("write geojson: %v", err)
		}
	}))
	t.Cleanup(server.Close)

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
				DistinctFlightAirportCodesFn: func(context.Context) ([]string, error) {
					return nil, nil
				},
				ListAirportIdentifiersByIATAFn: func(context.Context, []string) (map[string]store.AirportIdentifiers, error) {
					return map[string]store.AirportIdentifiers{}, nil
				},
				ReplaceWeatherStationsFn: func(context.Context, []string, [][]string, []string, [][]string) error {
					return nil
				},
				CreateRebuildRouteWeatherStatsJobFn: func(ctx context.Context) (*model.Job, error) {
					calls++

					return tt.create(ctx)
				},
			}

			catalog := iem.NewNetworkCatalog(server.URL, time.Second)
			h := NewWeatherStationsHandler(st, iem.NewService(st, nil).WithCatalog(catalog))
			_, err := h.Process(context.Background(), &model.Job{ID: testJobID})

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
