package operator

import (
	"context"
	"errors"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type Processor struct {
	store    store.Store
	handlers map[string]JobHandler
}

func NewProcessor(s store.Store, handlers ...JobHandler) *Processor {
	m := make(map[string]JobHandler, len(handlers))
	for _, h := range handlers {
		m[h.Type()] = h
	}

	return &Processor{
		store:    s,
		handlers: m,
	}
}

func (p *Processor) Process(ctx context.Context, job *model.Job) error {
	handler, ok := p.handlers[job.Type]
	if !ok {
		if err := p.store.FailJob(context.WithoutCancel(ctx), job.ID, fmt.Sprintf("unknown job type %q", job.Type)); err != nil {
			return fmt.Errorf("fail job: %w", err)
		}

		return fmt.Errorf("unknown job type %q", job.Type)
	}

	result, err := handler.Process(ctx, job)
	if err != nil {
		msg := err.Error()
		if errors.Is(context.Cause(ctx), ErrInterruptedByShutdown) {
			msg = ErrInterruptedByShutdown.Error()
		}

		if failErr := p.store.FailJob(context.WithoutCancel(ctx), job.ID, msg); failErr != nil && !errors.Is(failErr, store.ErrJobStatusConflict) {
			return fmt.Errorf("process: %w; fail job: %v", err, failErr)
		}

		return err
	}

	if err := p.store.CompleteJob(context.WithoutCancel(ctx), job.ID, result); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	return nil
}
