package operator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type Processor struct {
	store    store.Store
	handlers map[model.JobType]JobHandler
}

func NewProcessor(s store.Store, handlers ...JobHandler) (*Processor, error) {
	m := make(map[model.JobType]JobHandler, len(handlers))
	for _, h := range handlers {
		if _, exists := m[h.Type()]; exists {
			return nil, fmt.Errorf("duplicate job handler type %q", h.Type())
		}

		m[h.Type()] = h
	}

	return &Processor{
		store:    s,
		handlers: m,
	}, nil
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
		msg := persistedJobError(ctx, err)

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

func persistedJobError(ctx context.Context, err error) string {
	switch {
	case errors.Is(context.Cause(ctx), ErrInterruptedByShutdown):
		return ErrInterruptedByShutdown.Error()
	case errors.Is(context.Cause(ctx), ErrJobLeaseLost):
		return ErrJobLeaseLost.Error()
	default:
		return shortJobError(err)
	}
}

func shortJobError(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i > 0 {
		msg = msg[:i]
	}

	const maxLen = 160
	if len(msg) > maxLen {
		return msg[:maxLen]
	}

	return msg
}
