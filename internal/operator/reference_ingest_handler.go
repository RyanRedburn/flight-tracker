package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
)

type ReferenceIngestHandler struct {
	ingest  *ourairports.Service
	dataset model.ReferenceDataset
	jobType model.JobType
}

func NewCountriesHandler(ingest *ourairports.Service) *ReferenceIngestHandler {
	return newReferenceIngestHandler(ingest, model.ReferenceCountries, model.JobTypeImportCountries)
}

func NewRegionsHandler(ingest *ourairports.Service) *ReferenceIngestHandler {
	return newReferenceIngestHandler(ingest, model.ReferenceRegions, model.JobTypeImportRegions)
}

func NewAirportsHandler(ingest *ourairports.Service) *ReferenceIngestHandler {
	return newReferenceIngestHandler(ingest, model.ReferenceAirports, model.JobTypeImportAirports)
}

func newReferenceIngestHandler(
	ingest *ourairports.Service,
	dataset model.ReferenceDataset,
	jobType model.JobType,
) *ReferenceIngestHandler {
	return &ReferenceIngestHandler{
		ingest:  ingest,
		dataset: dataset,
		jobType: jobType,
	}
}

func (h *ReferenceIngestHandler) Type() model.JobType {
	return h.jobType
}

func (h *ReferenceIngestHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	result, err := h.ingest.Import(ctx, h.dataset)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
