package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type RebuildRouteTravelWindowsHandler struct {
	store store.Store
}

func NewRebuildRouteTravelWindowsHandler(s store.Store) *RebuildRouteTravelWindowsHandler {
	return &RebuildRouteTravelWindowsHandler{store: s}
}

func (h *RebuildRouteTravelWindowsHandler) Type() model.JobType {
	return model.JobTypeRebuildRouteTravelWindows
}

func (h *RebuildRouteTravelWindowsHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	if err := h.store.RebuildRouteTravelWindows(ctx); err != nil {
		return nil, fmt.Errorf("rebuild route travel windows: %w", err)
	}

	return json.RawMessage(`{"rebuilt":true}`), nil
}
