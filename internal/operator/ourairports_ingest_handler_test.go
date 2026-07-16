package operator

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestOurAirportsHandlerTypes(t *testing.T) {
	svc := ourairports.NewService(&storetest.Stub{}, nil)

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
