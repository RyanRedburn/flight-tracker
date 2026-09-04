package operator

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestWeatherStationsHandlerType(t *testing.T) {
	h := NewWeatherStationsHandler(iem.NewService(&storetest.Stub{}, nil))
	if got := h.Type(); got != model.JobTypeImportWeatherStations {
		t.Errorf("Type() = %q, want %q", got, model.JobTypeImportWeatherStations)
	}
}
