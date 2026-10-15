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

func (h *FlightPerformanceIngestHandler) Type() model.JobType {
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

	// The month COPY is already committed. Queue the same rollup jobs as
	// POST /api/internal/rebuild/travel-windows and /rebuild/weather-stats.
	// Each create holds the queue advisory lock and skips a type that is
	// already pending. A running rebuild does not block: it may predate this month.
	if err := enqueueRebuildJob(ctx, h.store.CreateRebuildRouteTravelWindowsJob); err != nil {
		return nil, fmt.Errorf("enqueue route travel window rebuild: %w", err)
	}

	if err := enqueueRebuildJob(ctx, h.store.CreateRebuildRouteWeatherStatsJob); err != nil {
		return nil, fmt.Errorf("enqueue route weather stats rebuild: %w", err)
	}

	return json.Marshal(result)
}
