package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type RebuildRouteWeatherStatsHandler struct {
	store store.Store
}

func NewRebuildRouteWeatherStatsHandler(s store.Store) *RebuildRouteWeatherStatsHandler {
	return &RebuildRouteWeatherStatsHandler{store: s}
}

func (h *RebuildRouteWeatherStatsHandler) Type() model.JobType {
	return model.JobTypeRebuildRouteWeatherStats
}

func (h *RebuildRouteWeatherStatsHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	if err := h.store.RebuildRouteWeatherStats(ctx); err != nil {
		return nil, fmt.Errorf("rebuild route weather stats: %w", err)
	}

	return json.RawMessage(`{"rebuilt":true}`), nil
}
