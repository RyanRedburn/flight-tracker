package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/mct"
	"github.com/RyanRedburn/flight-tracker/internal/model"
)

type AirportMCTHandler struct {
	ingest *mct.Service
}

func NewAirportMCTHandler(ingest *mct.Service) *AirportMCTHandler {
	return &AirportMCTHandler{ingest: ingest}
}

func (h *AirportMCTHandler) Type() model.JobType {
	return model.JobTypeImportAirportMCT
}

func (h *AirportMCTHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	result, err := h.ingest.Import(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
