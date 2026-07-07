package operator

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type Worker struct {
	store        store.Store
	processor    *Processor
	concurrency  int
	pollInterval time.Duration
	wg           sync.WaitGroup
	cancel       context.CancelFunc
	logger       *slog.Logger
}

func NewWorker(s store.Store, processor *Processor, concurrency int, pollInterval time.Duration, logger *slog.Logger) *Worker {
	if concurrency < 1 {
		concurrency = 1
	}

	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Worker{
		store:        s,
		processor:    processor,
		concurrency:  concurrency,
		pollInterval: pollInterval,
		logger:       logger,
	}
}

func (w *Worker) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.loop(runCtx, i)
	}
}

func (w *Worker) loop(ctx context.Context, workerID int) {
	defer w.wg.Done()

	logger := w.logger.With("worker", workerID)

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			w.poll(ctx, logger)
			timer.Reset(w.pollInterval)
		}
	}
}

func (w *Worker) poll(ctx context.Context, logger *slog.Logger) {
	job, err := w.store.ClaimNextPendingJob(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return
	}

	if err != nil {
		logger.Error("claim job failed", "error", err)
		return
	}

	if err := w.processor.Process(ctx, job); err != nil {
		logger.Error("job failed", "job_id", job.ID, "error", err)
		return
	}

	logger.Info("job completed", "job_id", job.ID)
}

func (w *Worker) Stop(timeout time.Duration) {
	if w.cancel != nil {
		w.cancel()
	}

	done := make(chan struct{})

	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		w.logger.Warn("worker shutdown timed out", "timeout", timeout)
	}
}
