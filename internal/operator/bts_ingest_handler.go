package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type BTSIngestHandler struct {
	store store.Store
}

func NewBTSIngestHandler(s store.Store) *BTSIngestHandler {
	return &BTSIngestHandler{store: s}
}

func (h *BTSIngestHandler) Type() string {
	return model.JobTypeImportBTSOnTime
}

func (h *BTSIngestHandler) Process(ctx context.Context, job *model.Job) (json.RawMessage, error) {
	detail, err := h.store.GetBTSIngestJob(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("get bts ingest job: %w", err)
	}

	return json.Marshal(map[string]any{
		"year":   detail.Year,
		"month":  detail.Month,
		"status": "pending_implementation",
	})
}
