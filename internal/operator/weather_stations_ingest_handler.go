package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
)

type WeatherStationsIngestHandler struct {
	ingest *iem.Service
}

func NewWeatherStationsHandler(ingest *iem.Service) *WeatherStationsIngestHandler {
	return &WeatherStationsIngestHandler{ingest: ingest}
}

func (h *WeatherStationsIngestHandler) Type() string {
	return model.JobTypeImportWeatherStations
}

func (h *WeatherStationsIngestHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	result, err := h.ingest.ImportStations(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}
