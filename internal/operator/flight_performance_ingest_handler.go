package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type FlightPerformanceIngestHandler struct {
	store  store.Store
	ingest *bts.Service
}

func NewFlightPerformanceIngestHandler(s store.Store, ingest *bts.Service) *FlightPerformanceIngestHandler {
	return &FlightPerformanceIngestHandler{store: s, ingest: ingest}
}

func (h *FlightPerformanceIngestHandler) Type() string {
	return model.JobTypeImportFlightPerformance
}

func (h *FlightPerformanceIngestHandler) Process(ctx context.Context, job *model.Job) (json.RawMessage, error) {
	detail, err := h.store.GetFlightPerformanceIngestJob(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("get flight performance ingest job: %w", err)
	}

	result, err := h.ingest.ImportMonth(ctx, detail.Year, detail.Month)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
