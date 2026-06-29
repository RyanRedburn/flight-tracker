package mem

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestStoreCreateGetUpdateList(t *testing.T) {
	s := New()
	ctx := context.Background()
	now := time.Now().UTC()

	job := &model.Job{
		ID:        "job-1",
		Type:      model.JobTypeFetchFlights,
		Payload:   json.RawMessage(`{"foo":"bar"}`),
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	got, err := s.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if string(got.Payload) != string(job.Payload) {
		t.Errorf("Payload = %s, want %s", got.Payload, job.Payload)
	}

	got.Status = model.JobStatusCompleted

	got.Result = json.RawMessage(`{"ok":true}`)
	if err := s.UpdateJob(ctx, got); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}

	stored, err := s.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() after update error = %v", err)
	}

	if stored.Status != model.JobStatusCompleted {
		t.Errorf("Status = %q, want completed", stored.Status)
	}

	jobs, err := s.ListJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
}

func TestStoreGetJobNotFound(t *testing.T) {
	_, err := New().GetJob(context.Background(), "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetJob() error = %v, want ErrNotFound", err)
	}
}

func TestStorePingHook(t *testing.T) {
	s := New()
	pingErr := errors.New("db down")

	s.SetPingHook(func(context.Context) error { return pingErr })

	if err := s.Ping(context.Background()); !errors.Is(err, pingErr) {
		t.Fatalf("Ping() error = %v, want %v", err, pingErr)
	}
}

func TestStoreListJobsDefaultLimit(t *testing.T) {
	s := New()
	ctx := context.Background()
	now := time.Now().UTC()

	for i := range 3 {
		job := &model.Job{
			ID:        "job-" + string(rune('a'+i)),
			Type:      model.JobTypeFetchFlights,
			Status:    model.JobStatusPending,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now,
		}
		if err := s.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
	}

	jobs, err := s.ListJobs(ctx, 0)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("len(jobs) = %d, want 3", len(jobs))
	}

	if jobs[0].ID != "job-c" {
		t.Errorf("first job ID = %q, want job-c (newest first)", jobs[0].ID)
	}
}
