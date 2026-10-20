package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type WeatherStationsIngestHandler struct {
	store  store.Store
	ingest *iem.Service
}

func NewWeatherStationsHandler(s store.Store, ingest *iem.Service) *WeatherStationsIngestHandler {
	return &WeatherStationsIngestHandler{store: s, ingest: ingest}
}

func (h *WeatherStationsIngestHandler) Type() model.JobType {
	return model.JobTypeImportWeatherStations
}

func (h *WeatherStationsIngestHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	result, err := h.ingest.ImportStations(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
