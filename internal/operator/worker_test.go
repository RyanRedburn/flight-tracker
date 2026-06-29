package operator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/source/mock"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func TestWorkerProcessesSubmittedJob(t *testing.T) {
	store := mem.New()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "worker-job-1",
		Type:      model.JobTypeFetchFlights,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	provider := &mock.Provider{Latency: 0}
	processor := NewProcessor(store, provider)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := NewWorker(processor, 1, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop(5 * time.Second)

	worker.Submit(job.ID)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := store.GetJob(context.Background(), job.ID)
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
	processor := NewProcessor(mem.New(), &mock.Provider{Latency: 0})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	worker := NewWorker(processor, 0, logger)
	if worker.concurrency != 1 {
		t.Errorf("concurrency = %d, want 1", worker.concurrency)
	}
}
