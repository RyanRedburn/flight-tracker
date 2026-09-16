package operator

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const jobStatusWriteTimeout = 5 * time.Second

var ErrInterruptedByShutdown = errors.New("interrupted by shutdown")

type WorkerConfig struct {
	Concurrency       int
	PollInterval      time.Duration
	LeaseTTL          time.Duration
	HeartbeatInterval time.Duration
	ReclaimInterval   time.Duration
}

type inFlightJob struct {
	id     string
	cancel context.CancelCauseFunc
}

type Worker struct {
	store             store.Store
	processor         *Processor
	concurrency       int
	pollInterval      time.Duration
	leaseTTL          time.Duration
	heartbeatInterval time.Duration
	reclaimInterval   time.Duration
	wg                sync.WaitGroup
	claimCancel       context.CancelFunc
	shutdownOnce      sync.Once
	mu                sync.Mutex
	inFlight          map[int]inFlightJob
	logger            *slog.Logger
}

func NewWorker(s store.Store, processor *Processor, cfg WorkerConfig, logger *slog.Logger) *Worker {
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}

	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}

	if cfg.LeaseTTL > 0 {
		if cfg.HeartbeatInterval <= 0 {
			cfg.HeartbeatInterval = cfg.LeaseTTL / 3
		}

		if cfg.ReclaimInterval <= 0 {
			cfg.ReclaimInterval = cfg.LeaseTTL / 3
		}
	}

	return &Worker{
		store:             s,
		processor:         processor,
		concurrency:       cfg.Concurrency,
		pollInterval:      cfg.PollInterval,
		leaseTTL:          cfg.LeaseTTL,
		heartbeatInterval: cfg.HeartbeatInterval,
		reclaimInterval:   cfg.ReclaimInterval,
		inFlight:          make(map[int]inFlightJob),
		logger:            logger,
	}
}

func (w *Worker) Start(ctx context.Context) {
	claimCtx, cancel := context.WithCancel(ctx)
	w.claimCancel = cancel

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.loop(claimCtx, i)
	}

	if w.reclaimInterval > 0 && w.leaseTTL > 0 {
		w.wg.Add(1)
		go w.reclaimLoop(claimCtx)
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
			w.poll(ctx, logger, workerID)
			timer.Reset(w.pollInterval)
		}
	}
}

func (w *Worker) poll(ctx context.Context, logger *slog.Logger, workerID int) {
	if ctx.Err() != nil {
		return
	}

	var leaseUntil time.Time
	if w.leaseTTL > 0 {
		leaseUntil = time.Now().UTC().Add(w.leaseTTL)
	}

	job, err := w.store.ClaimNextPendingJob(ctx, leaseUntil)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, context.Canceled) {
		return
	}

	if err != nil {
		logger.Error("claim job failed", "error", err)

		return
	}

	processCtx, cancel := context.WithCancelCause(context.Background())

	w.mu.Lock()

	w.inFlight[workerID] = inFlightJob{id: job.ID, cancel: cancel}
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()

		delete(w.inFlight, workerID)
		w.mu.Unlock()
	}()

	if ctx.Err() != nil {
		w.failInterrupted(job.ID)

		cancel(ErrInterruptedByShutdown)

		return
	}

	stopHB := make(chan struct{})
	doneHB := make(chan struct{})

	if w.leaseTTL > 0 && w.heartbeatInterval > 0 {
		go func() {
			defer close(doneHB)

			w.runHeartbeat(stopHB, job.ID, logger, cancel)
		}()
	} else {
		close(doneHB)
	}

	err = w.processor.Process(processCtx, job)

	close(stopHB)
	<-doneHB

	if err != nil {
		logger.Error("job failed", "job_id", job.ID, "error", err)

		return
	}

	logger.Info("job completed", "job_id", job.ID)
}

func (w *Worker) runHeartbeat(stop <-chan struct{}, jobID string, logger *slog.Logger, abort context.CancelCauseFunc) {
	ticker := time.NewTicker(w.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			leaseUntil := time.Now().UTC().Add(w.leaseTTL)

			hbCtx, cancel := context.WithTimeout(context.Background(), w.heartbeatInterval)
			err := w.store.HeartbeatJob(hbCtx, jobID, leaseUntil)

			cancel()

			if err == nil {
				continue
			}

			logger.Error("job heartbeat failed", "job_id", jobID, "error", err)

			if errors.Is(err, store.ErrJobStatusConflict) {
				abort(err)

				return
			}
		}
	}
}

func (w *Worker) reclaimLoop(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.reclaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := RecoverStaleJobs(ctx, w.store, w.leaseTTL, w.logger); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				w.logger.Error("reclaim expired job leases", "error", err)
			}
		}
	}
}

func (w *Worker) failInterrupted(jobID string) {
	failCtx, cancel := context.WithTimeout(context.Background(), jobStatusWriteTimeout)
	defer cancel()

	if err := w.store.FailJob(failCtx, jobID, ErrInterruptedByShutdown.Error()); err != nil && !errors.Is(err, store.ErrJobStatusConflict) {
		w.logger.Error("fail job on shutdown", "job_id", jobID, "error", err)
	}
}

func (w *Worker) Shutdown() {
	w.shutdownOnce.Do(func() {
		if w.claimCancel != nil {
			w.claimCancel()
		}

		w.mu.Lock()

		inflight := make([]inFlightJob, 0, len(w.inFlight))
		for _, job := range w.inFlight {
			inflight = append(inflight, job)
		}

		w.mu.Unlock()

		for _, job := range inflight {
			w.failInterrupted(job.id)

			if job.cancel != nil {
				job.cancel(ErrInterruptedByShutdown)
			}
		}
	})
}

func (w *Worker) Stop(timeout time.Duration) {
	w.Shutdown()

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
