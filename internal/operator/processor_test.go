package operator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

const testJobType = "test_job"

type testHandler struct {
	jobType string
	result  json.RawMessage
	err     error
}

func (h testHandler) Type() string { return h.jobType }

func (h testHandler) Process(ctx context.Context, job *model.Job) (json.RawMessage, error) {
	if h.err != nil {
		return nil, h.err
	}

	return h.result, nil
}

func TestProcessorProcessSuccess(t *testing.T) {
	store := mem.New()
	ctx := context.Background()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "job-1",
		Type:      testJobType,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	result := json.RawMessage(`{"ok":true}`)
	processor := NewProcessor(store, testHandler{jobType: testJobType, result: result})

	claimed, err := store.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if err := processor.Process(ctx, claimed); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}

	if string(got.Result) != string(result) {
		t.Errorf("Result = %s, want %s", got.Result, result)
	}

	if got.EndedAt == nil {
		t.Error("EndedAt is nil, want timestamp")
	}
}

func TestProcessorProcessHandlerFailure(t *testing.T) {
	store := mem.New()
	ctx := context.Background()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "job-2",
		Type:      testJobType,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	handlerErr := errors.New("handler failed")
	processor := NewProcessor(store, testHandler{jobType: testJobType, err: handlerErr})

	claimed, err := store.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if err := processor.Process(ctx, claimed); err == nil {
		t.Fatal("Process() expected error")
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}

	if got.Error != handlerErr.Error() {
		t.Errorf("Error = %q, want %q", got.Error, handlerErr.Error())
	}
}

func TestProcessorProcessUnknownJobType(t *testing.T) {
	store := mem.New()
	ctx := context.Background()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "job-unknown",
		Type:      "unknown_type",
		Status:    model.JobStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
		StartedAt: &now,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	processor := NewProcessor(store)

	if err := processor.Process(ctx, job); err == nil {
		t.Fatal("Process() expected error")
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}
}

func TestBTSIngestHandlerImportsCSV(t *testing.T) {
	store := mem.New()
	ctx := context.Background()

	job, err := store.CreateBTSIngestJob(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	claimed, err := store.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if claimed.ID != job.ID {
		t.Fatalf("claimed ID = %q, want %q", claimed.ID, job.ID)
	}

	processor := NewProcessor(store, newTestBTSIngestHandler(t, store))
	if err := processor.Process(ctx, claimed); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}

	var result map[string]any
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if result["rows_imported"] == nil || result["rows_imported"].(float64) != 1 {
		t.Fatalf("result = %v, want rows_imported = 1", result)
	}
}
