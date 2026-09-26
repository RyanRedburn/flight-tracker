package operator

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
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
