package operator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestRebuildRouteTravelWindowsHandlerProcess(t *testing.T) {
	ctx := context.Background()

	calls := 0
	h := NewRebuildRouteTravelWindowsHandler(&storetest.Stub{
		RebuildRouteTravelWindowsFn: func(context.Context) error {
			calls++

			return nil
		},
	})

	if got := h.Type(); got != model.JobTypeRebuildRouteTravelWindows {
		t.Fatalf("Type() = %q, want %q", got, model.JobTypeRebuildRouteTravelWindows)
	}

	payload, err := h.Process(ctx, &model.Job{ID: testJobID, Type: h.Type()})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("RebuildRouteTravelWindows calls = %d, want 1", calls)
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	rebuilt, ok := result["rebuilt"].(bool)
	if !ok || !rebuilt {
		t.Fatalf("result = %v, want rebuilt true", result)
	}
}

func TestRebuildRouteTravelWindowsHandlerError(t *testing.T) {
	ctx := context.Background()
	h := NewRebuildRouteTravelWindowsHandler(&storetest.Stub{
		RebuildRouteTravelWindowsFn: func(context.Context) error {
			return errRebuildFailed
		},
	})

	if _, err := h.Process(ctx, &model.Job{ID: testJobID, Type: h.Type()}); err == nil {
		t.Fatal("expected rebuild error")
	}
}
