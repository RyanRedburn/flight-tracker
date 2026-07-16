package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type OurAirportsIngestHandler struct {
	ingest  *ourairports.Service
	dataset store.OurAirportsDataset
	jobType string
}

func NewCountriesHandler(ingest *ourairports.Service) *OurAirportsIngestHandler {
	return newOurAirportsIngestHandler(ingest, store.OurAirportsCountries, model.JobTypeImportOurAirportsCountries)
}

func NewRegionsHandler(ingest *ourairports.Service) *OurAirportsIngestHandler {
	return newOurAirportsIngestHandler(ingest, store.OurAirportsRegions, model.JobTypeImportOurAirportsRegions)
}

func NewAirportsHandler(ingest *ourairports.Service) *OurAirportsIngestHandler {
	return newOurAirportsIngestHandler(ingest, store.OurAirportsAirports, model.JobTypeImportOurAirportsAirports)
}

func newOurAirportsIngestHandler(
	ingest *ourairports.Service,
	dataset store.OurAirportsDataset,
	jobType string,
) *OurAirportsIngestHandler {
	return &OurAirportsIngestHandler{
		ingest:  ingest,
		dataset: dataset,
		jobType: jobType,
	}
}

func (h *OurAirportsIngestHandler) Type() string {
	return h.jobType
}

func (h *OurAirportsIngestHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	result, err := h.ingest.Import(ctx, h.dataset)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
