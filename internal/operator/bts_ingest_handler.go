package operator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type BTSIngestHandler struct {
	store  store.Store
	ingest *bts.Service
}

func NewBTSIngestHandler(s store.Store, ingest *bts.Service) *BTSIngestHandler {
	return &BTSIngestHandler{store: s, ingest: ingest}
}

func (h *BTSIngestHandler) Type() string {
	return model.JobTypeImportBTSOnTime
}

func (h *BTSIngestHandler) Process(ctx context.Context, job *model.Job) (json.RawMessage, error) {
	detail, err := h.store.GetBTSIngestJob(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("get bts ingest job: %w", err)
	}

	result, err := h.ingest.ImportMonth(ctx, detail.Year, detail.Month)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
