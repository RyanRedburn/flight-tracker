package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type ReferenceIngestHandler struct {
	ingest  *ourairports.Service
	dataset store.ReferenceDataset
	jobType string
}

func NewCountriesHandler(ingest *ourairports.Service) *ReferenceIngestHandler {
	return newReferenceIngestHandler(ingest, store.ReferenceCountries, model.JobTypeImportCountries)
}

func NewRegionsHandler(ingest *ourairports.Service) *ReferenceIngestHandler {
	return newReferenceIngestHandler(ingest, store.ReferenceRegions, model.JobTypeImportRegions)
}

func NewAirportsHandler(ingest *ourairports.Service) *ReferenceIngestHandler {
	return newReferenceIngestHandler(ingest, store.ReferenceAirports, model.JobTypeImportAirports)
}

func newReferenceIngestHandler(
	ingest *ourairports.Service,
	dataset store.ReferenceDataset,
	jobType string,
) *ReferenceIngestHandler {
	return &ReferenceIngestHandler{
		ingest:  ingest,
		dataset: dataset,
		jobType: jobType,
	}
}

func (h *ReferenceIngestHandler) Type() string {
	return h.jobType
}

func (h *ReferenceIngestHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	result, err := h.ingest.Import(ctx, h.dataset)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
