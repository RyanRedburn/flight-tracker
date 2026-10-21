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

func TestWeatherStationsHandlerImportError(t *testing.T) {
	st := &storetest.Stub{
		CreateRebuildRouteWeatherStatsJobFn: rejectRebuild(t),
	}
	h := NewWeatherStationsHandler(st, iem.NewService(st, nil))

	if _, err := h.Process(context.Background(), &model.Job{ID: testJobID}); err == nil {
		t.Fatal("expected import error")
	}
}

func TestWeatherStationsHandlerDoesNotQueueRebuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"features":[{"properties":{"sid":"ORD","network":"IL_ASOS","tzname":"America/Chicago"}}]}`)); err != nil {
			t.Errorf("write geojson: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	replaced := false
	st := &storetest.Stub{
		DistinctFlightAirportCodesFn: func(context.Context) ([]string, error) {
			return nil, nil
		},
		ListAirportIdentifiersByIATAFn: func(context.Context, []string) (map[string]store.AirportIdentifiers, error) {
			return map[string]store.AirportIdentifiers{}, nil
		},
		ReplaceWeatherStationsFn: func(context.Context, []string, [][]string, []string, [][]string) error {
			replaced = true

			return nil
		},
		CreateRebuildRouteWeatherStatsJobFn: rejectRebuild(t),
	}

	catalog := iem.NewNetworkCatalog(server.URL, time.Second)

	h := NewWeatherStationsHandler(st, iem.NewService(st, nil).WithCatalog(catalog))
	if _, err := h.Process(context.Background(), &model.Job{ID: testJobID}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if !replaced {
		t.Fatal("expected weather stations to be replaced")
	}
}
