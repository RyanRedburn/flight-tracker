package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/source"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type Processor struct {
	store    store.Store
	provider source.Provider
}

func NewProcessor(s store.Store, provider source.Provider) *Processor {
	return &Processor{
		store:    s,
		provider: provider,
	}
}

func (p *Processor) Process(ctx context.Context, jobID string) error {
	job, err := p.store.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	now := time.Now().UTC()
	job.Status = model.JobStatusRunning
	job.UpdatedAt = now

	if err := p.store.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("mark running: %w", err)
	}

	result, err := p.provider.Fetch(ctx, source.FetchRequest{Params: json.RawMessage(`{}`)})
	now = time.Now().UTC()
	job.UpdatedAt = now

	if err != nil {
		job.Status = model.JobStatusFailed
		job.Error = err.Error()

		if updateErr := p.store.UpdateJob(ctx, job); updateErr != nil {
			return fmt.Errorf("fetch failed: %w; update job: %v", err, updateErr)
		}

		return fmt.Errorf("fetch: %w", err)
	}

	job.Status = model.JobStatusCompleted
	job.Result = result
	job.Error = ""

	if err := p.store.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}

	return nil
}
