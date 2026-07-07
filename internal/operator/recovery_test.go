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

func TestRecoverStaleJobs(t *testing.T) {
	store := mem.New()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	started := time.Now().UTC().Add(-2 * time.Hour)

	job := &model.Job{
		ID:        "stale-job",
		Type:      model.JobTypeImportBTSOnTime,
		Status:    model.JobStatusRunning,
		CreatedAt: started,
		UpdatedAt: started,
		StartedAt: &started,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if err := RecoverStaleJobs(ctx, store, time.Hour, logger); err != nil {
		t.Fatalf("RecoverStaleJobs() error = %v", err)
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}

	if got.StartedAt != nil {
		t.Error("StartedAt should be cleared")
	}
}
