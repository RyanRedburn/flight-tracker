package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type RollupRebuildHandler struct {
	jobType model.JobType
	rebuild func(context.Context) error
	action  string
}

func NewRebuildRouteTravelWindowsHandler(s store.Store) *RollupRebuildHandler {
	return &RollupRebuildHandler{
		jobType: model.JobTypeRebuildRouteTravelWindows,
		rebuild: s.RebuildRouteTravelWindows,
		action:  "rebuild route travel windows",
	}
}

func NewRebuildRouteWeatherStatsHandler(s store.Store) *RollupRebuildHandler {
	return &RollupRebuildHandler{
		jobType: model.JobTypeRebuildRouteWeatherStats,
		rebuild: s.RebuildRouteWeatherStats,
		action:  "rebuild route weather stats",
	}
}

func (h *RollupRebuildHandler) Type() model.JobType {
	return h.jobType
}

func (h *RollupRebuildHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	if err := h.rebuild(ctx); err != nil {
		return nil, fmt.Errorf("%s: %w", h.action, err)
	}

	return json.RawMessage(`{"rebuilt":true}`), nil
}
