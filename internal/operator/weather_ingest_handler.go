package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type WeatherIngestHandler struct {
	store  store.Store
	ingest *iem.Service
}

func NewWeatherIngestHandler(s store.Store, ingest *iem.Service) *WeatherIngestHandler {
	return &WeatherIngestHandler{store: s, ingest: ingest}
}

func (h *WeatherIngestHandler) Type() model.JobType {
	return model.JobTypeImportWeatherObservations
}

func (h *WeatherIngestHandler) Process(ctx context.Context, job *model.Job) (json.RawMessage, error) {
	detail, err := h.store.GetWeatherIngestJob(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("get weather ingest job: %w", err)
	}

	result, err := h.ingest.ImportMonth(ctx, detail.Year, detail.Month, detail.Stations)
	if err != nil {
		return nil, err
	}

	// Observations are committed. Rebuild route weather rollups so the nearest
	// ±30 minute match uses the new month. The advisory lock serializes replicas.
	if err := h.store.RebuildRouteWeatherStats(ctx); err != nil {
		return nil, fmt.Errorf("rebuild route weather stats: %w", err)
	}

	return json.Marshal(result)
}
