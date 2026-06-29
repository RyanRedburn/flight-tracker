package operator

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Worker struct {
	processor   *Processor
	concurrency int
	queue       chan string
	wg          sync.WaitGroup
	cancel      context.CancelFunc
	logger      *slog.Logger
}

func NewWorker(processor *Processor, concurrency int, logger *slog.Logger) *Worker {
	if concurrency < 1 {
		concurrency = 1
	}

	return &Worker{
		processor:   processor,
		concurrency: concurrency,
		queue:       make(chan string, 256),
		logger:      logger,
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

	for {
		select {
		case <-ctx.Done():
			return
		case jobID, ok := <-w.queue:
			if !ok {
				return
			}

			if err := w.processor.Process(ctx, jobID); err != nil {
				logger.Error("job failed", "job_id", jobID, "error", err)
			} else {
				logger.Info("job completed", "job_id", jobID)
			}
		}
	}
}

func (w *Worker) Submit(jobID string) {
	w.queue <- jobID
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
