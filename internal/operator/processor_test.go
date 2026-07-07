package operator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/source"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

type stubProvider struct {
	result json.RawMessage
	err    error
}

func (p stubProvider) Fetch(ctx context.Context, req source.FetchRequest) (json.RawMessage, error) {
	if p.err != nil {
		return nil, p.err
	}

	return p.result, nil
}

func TestProcessorProcessSuccess(t *testing.T) {
	store := mem.New()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "job-1",
		Type:      model.JobTypeFetchFlights,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	result := json.RawMessage(`[{"callsign":"TEST"}]`)
	processor := NewProcessor(store, stubProvider{result: result})

	if err := processor.Process(context.Background(), job.ID); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}

	if string(got.Result) != string(result) {
		t.Errorf("Result = %s, want %s", got.Result, result)
	}
}

func TestProcessorProcessFetchFailure(t *testing.T) {
	store := mem.New()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "job-2",
		Type:      model.JobTypeFetchFlights,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	fetchErr := errors.New("upstream unavailable")
	processor := NewProcessor(store, stubProvider{err: fetchErr})

	if err := processor.Process(context.Background(), job.ID); err == nil {
		t.Fatal("Process() expected error")
	}

	got, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if got.Status != model.JobStatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}

	if got.Error != fetchErr.Error() {
		t.Errorf("Error = %q, want %q", got.Error, fetchErr.Error())
	}
}

func TestProcessorProcessJobNotFound(t *testing.T) {
	processor := NewProcessor(mem.New(), stubProvider{result: json.RawMessage(`[]`)})

	err := processor.Process(context.Background(), "missing")
	if err == nil {
		t.Fatal("Process() expected error for missing job")
	}
}
