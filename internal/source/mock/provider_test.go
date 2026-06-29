package mock

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/source"
)

func TestFetchReturnsAircraft(t *testing.T) {
	p := &Provider{Latency: 0}

	data, err := p.Fetch(context.Background(), source.FetchRequest{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	var aircraft []map[string]any
	if err := json.Unmarshal(data, &aircraft); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(aircraft) != 2 {
		t.Fatalf("len(aircraft) = %d, want 2", len(aircraft))
	}

	if aircraft[0]["callsign"] != "MOCK001" {
		t.Errorf("callsign = %v, want MOCK001", aircraft[0]["callsign"])
	}
}

func TestFetchRespectsContextCancellation(t *testing.T) {
	p := &Provider{Latency: time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Fetch(ctx, source.FetchRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context.Canceled", err)
	}
}
