package operator

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestReferenceHandlerTypes(t *testing.T) {
	svc := ourairports.NewService(&storetest.Stub{}, nil)

	tests := []struct {
		name    string
		handler JobHandler
		want    string
	}{
		{"countries", NewCountriesHandler(svc), model.JobTypeImportCountries},
		{"regions", NewRegionsHandler(svc), model.JobTypeImportRegions},
		{"airports", NewAirportsHandler(svc), model.JobTypeImportAirports},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.handler.Type(); got != tt.want {
				t.Errorf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}
