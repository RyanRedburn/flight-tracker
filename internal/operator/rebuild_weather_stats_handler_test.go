package operator

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestRebuildRouteWeatherStatsHandlerProcess(t *testing.T) {
	ctx := context.Background()

	calls := 0
	h := NewRebuildRouteWeatherStatsHandler(&storetest.Stub{
		RebuildRouteWeatherStatsFn: func(context.Context) error {
			calls++

			return nil
		},
	})

	if got := h.Type(); got != model.JobTypeRebuildRouteWeatherStats {
		t.Fatalf("Type() = %q, want %q", got, model.JobTypeRebuildRouteWeatherStats)
	}

	payload, err := h.Process(ctx, &model.Job{ID: testJobID, Type: h.Type()})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("RebuildRouteWeatherStats calls = %d, want 1", calls)
	}

	if string(payload) != `{"rebuilt":true}` {
		t.Fatalf("payload = %s", payload)
	}
}

func TestRebuildRouteWeatherStatsHandlerError(t *testing.T) {
	ctx := context.Background()
	h := NewRebuildRouteWeatherStatsHandler(&storetest.Stub{
		RebuildRouteWeatherStatsFn: func(context.Context) error {
			return errRebuildFailed
		},
	})

	if _, err := h.Process(ctx, &model.Job{ID: testJobID, Type: h.Type()}); err == nil {
		t.Fatal("expected rebuild error")
	}
}
