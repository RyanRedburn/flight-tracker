package mock

import (
	"context"
	"encoding/json"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/source"
)

type Provider struct {
	Latency time.Duration
}

func New() *Provider {
	return &Provider{Latency: 500 * time.Millisecond}
}

func (p *Provider) Fetch(ctx context.Context, req source.FetchRequest) (json.RawMessage, error) {
	if p.Latency > 0 {
		timer := time.NewTimer(p.Latency)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	payload := []map[string]any{
		{
			"icao24":    "abc123",
			"callsign":  "MOCK001",
			"latitude":  37.6213,
			"longitude": -122.3790,
			"altitude":  10500,
			"velocity":  220.5,
			"heading":   270,
		},
		{
			"icao24":    "def456",
			"callsign":  "MOCK002",
			"latitude":  40.6413,
			"longitude": -73.7781,
			"altitude":  8500,
			"velocity":  195.0,
			"heading":   90,
		},
	}

	return json.Marshal(payload)
}
