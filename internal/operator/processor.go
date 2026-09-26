package operator

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	// jobErrorMaxLen caps text stored in jobs.error. The column is unbounded;
	// longer messages keep their head and tail so the operation label and the
	// causal suffix both remain visible.
	jobErrorMaxLen = 512

	jobErrorOmit = "..."
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
	if len(msg) <= jobErrorMaxLen {
		return msg
	}

	budget := jobErrorMaxLen - len(jobErrorOmit)
	headLen := budget / 2
	tailLen := budget - headLen

	return trimRightToRune(msg[:headLen]) + jobErrorOmit + trimLeftToRune(msg[len(msg)-tailLen:])
}

// trimRightToRune drops a trailing partial rune left by a byte cut.
func trimRightToRune(s string) string {
	for range utf8.UTFMax - 1 {
		if s == "" || utf8.ValidString(s) {
			return s
		}

		s = s[:len(s)-1]
	}

	return s
}

// trimLeftToRune drops a leading partial rune left by a byte cut.
func trimLeftToRune(s string) string {
	for range utf8.UTFMax - 1 {
		if s == "" || utf8.ValidString(s) {
			return s
		}

		s = s[1:]
	}

	return s
}
