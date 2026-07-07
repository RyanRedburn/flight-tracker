package operator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func TestWorkerPollsPendingJob(t *testing.T) {
	store := mem.New()
	ctx := context.Background()

	job, err := store.CreateBTSIngestJob(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	processor := NewProcessor(store, newTestBTSIngestHandler(t, store))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := NewWorker(store, processor, 1, 10*time.Millisecond, logger)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	worker.Start(runCtx)
	defer worker.Stop(5 * time.Second)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := store.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}

		if got.Status == model.JobStatusCompleted {
			if len(got.Result) == 0 {
				t.Fatal("expected non-empty result")
			}

			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("job was not completed before timeout")
}

func TestNewWorkerMinimumConcurrency(t *testing.T) {
	processor := NewProcessor(mem.New(), newTestBTSIngestHandler(t, mem.New()))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	worker := NewWorker(mem.New(), processor, 0, time.Second, logger)
	if worker.concurrency != 1 {
		t.Errorf("concurrency = %d, want 1", worker.concurrency)
	}
}
