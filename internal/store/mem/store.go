package mem

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type Store struct {
	mu   sync.Mutex
	jobs map[string]*model.Job
	ping func(context.Context) error
}

func New() *Store {
	return &Store{
		jobs: make(map[string]*model.Job),
		ping: func(context.Context) error { return nil },
	}
}

func (s *Store) SetPingHook(fn func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ping = fn
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	s.mu.Lock()
	fn := s.ping
	s.mu.Unlock()

	return fn(ctx)
}

func (s *Store) MigrationVersion(context.Context) (store.MigrationVersion, error) {
	return store.MigrationVersion{}, nil
}

func (s *Store) CreateJob(ctx context.Context, job *model.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job %q already exists", job.ID)
	}

	s.jobs[job.ID] = cloneJob(job)

	return nil
}

func (s *Store) GetJob(ctx context.Context, id string) (*model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q: %w", id, store.ErrNotFound)
	}

	return cloneJob(job), nil
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]*model.Job, error) {
	if limit <= 0 {
		limit = 50
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	jobs := make([]*model.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, cloneJob(job))
	}

	slices.SortFunc(jobs, func(a, b *model.Job) int {
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}

		if a.CreatedAt.Before(b.CreatedAt) {
			return 1
		}

		return 0
	})

	if len(jobs) > limit {
		jobs = jobs[:limit]
	}

	return jobs, nil
}

func (s *Store) UpdateJob(ctx context.Context, job *model.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[job.ID]; !ok {
		return fmt.Errorf("job %q: %w", job.ID, store.ErrNotFound)
	}

	s.jobs[job.ID] = cloneJob(job)

	return nil
}

func cloneJob(job *model.Job) *model.Job {
	jobCopy := *job
	if job.Payload != nil {
		jobCopy.Payload = append(json.RawMessage(nil), job.Payload...)
	}

	if job.Result != nil {
		jobCopy.Result = append(json.RawMessage(nil), job.Result...)
	}

	return &jobCopy
}
