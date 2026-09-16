package operator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

type failCall struct {
	id  string
	msg string
}

type blockingHandler struct {
	started chan struct{}
}

func (h blockingHandler) Type() string { return testJobType }

func (h blockingHandler) Process(ctx context.Context, _ *model.Job) (json.RawMessage, error) {
	close(h.started)
	<-ctx.Done()

	return nil, ctx.Err()
}

func TestNewWorkerMinimumConcurrency(t *testing.T) {
	st := &storetest.Stub{}
	processor := NewProcessor(st)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	worker := NewWorker(st, processor, WorkerConfig{Concurrency: 0, PollInterval: time.Second}, logger)
	if worker.concurrency != 1 {
		t.Errorf("concurrency = %d, want 1", worker.concurrency)
	}
}

func TestNewWorkerDerivedLeaseIntervals(t *testing.T) {
	st := &storetest.Stub{}
	processor := NewProcessor(st)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	worker := NewWorker(st, processor, WorkerConfig{LeaseTTL: 90 * time.Second}, logger)
	if worker.heartbeatInterval != 30*time.Second {
		t.Errorf("heartbeatInterval = %v, want 30s", worker.heartbeatInterval)
	}

	if worker.reclaimInterval != 30*time.Second {
		t.Errorf("reclaimInterval = %v, want 30s", worker.reclaimInterval)
	}
}

func TestWorkerStopFailsInFlightJobWithLiveContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	started := make(chan struct{})

	var (
		mu        sync.Mutex
		failCalls []failCall
	)

	st := &storetest.Stub{
		ClaimNextPendingJobFn: func(_ context.Context, _ time.Time) (*model.Job, error) {
			return &model.Job{ID: testJobID, Type: testJobType, Status: model.JobStatusRunning}, nil
		},
		FailJobFn: func(ctx context.Context, id, errMsg string) error {
			if err := ctx.Err(); err != nil {
				t.Errorf("FailJob ctx already done: %v", err)
			}

			mu.Lock()

			failCalls = append(failCalls, failCall{id: id, msg: errMsg})
			mu.Unlock()

			return nil
		},
	}

	processor := NewProcessor(st, blockingHandler{started: started})
	worker := NewWorker(st, processor, WorkerConfig{
		Concurrency:       1,
		PollInterval:      time.Hour,
		LeaseTTL:          time.Minute,
		HeartbeatInterval: time.Hour,
		ReclaimInterval:   0,
	}, logger)

	worker.Start(context.Background())

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Process to start")
	}

	worker.Stop(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	if len(failCalls) == 0 {
		t.Fatal("expected FailJob on shutdown")
	}

	found := false

	for _, call := range failCalls {
		if call.id == testJobID && call.msg == ErrInterruptedByShutdown.Error() {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("FailJob calls = %+v, want id %q msg %q", failCalls, testJobID, ErrInterruptedByShutdown.Error())
	}
}

func TestWorkerStopIdleExits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	st := &storetest.Stub{
		ClaimNextPendingJobFn: func(ctx context.Context, _ time.Time) (*model.Job, error) {
			<-ctx.Done()

			return nil, ctx.Err()
		},
	}

	worker := NewWorker(st, NewProcessor(st), WorkerConfig{
		Concurrency:     1,
		PollInterval:    time.Hour,
		ReclaimInterval: 0,
	}, logger)

	worker.Start(context.Background())
	worker.Stop(2 * time.Second)
}

func TestWorkerShutdownStopsClaiming(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var claims atomic.Int32

	st := &storetest.Stub{
		ClaimNextPendingJobFn: func(ctx context.Context, _ time.Time) (*model.Job, error) {
			claims.Add(1)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(20 * time.Millisecond):
				return nil, store.ErrNotFound
			}
		},
	}

	worker := NewWorker(st, NewProcessor(st), WorkerConfig{
		Concurrency:     1,
		PollInterval:    15 * time.Millisecond,
		ReclaimInterval: 0,
	}, logger)

	worker.Start(context.Background())
	time.Sleep(40 * time.Millisecond)
	worker.Shutdown()

	afterShutdown := claims.Load()

	time.Sleep(80 * time.Millisecond)

	if got := claims.Load(); got > afterShutdown+1 {
		t.Errorf("claims kept increasing after Shutdown: before=%d after=%d", afterShutdown, got)
	}

	worker.Stop(2 * time.Second)
}

func TestWorkerHeartbeatWhileProcessInFlight(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	started := make(chan struct{})
	heartbeats := make(chan time.Time, 8)

	st := &storetest.Stub{
		ClaimNextPendingJobFn: func(_ context.Context, _ time.Time) (*model.Job, error) {
			return &model.Job{ID: testJobID, Type: testJobType, Status: model.JobStatusRunning}, nil
		},
		HeartbeatJobFn: func(ctx context.Context, id string, leaseUntil time.Time) error {
			if err := ctx.Err(); err != nil {
				t.Errorf("HeartbeatJob ctx already done: %v", err)
			}

			if id != testJobID {
				t.Errorf("HeartbeatJob id = %q, want %q", id, testJobID)
			}

			select {
			case heartbeats <- leaseUntil:
			default:
			}

			return nil
		},
		FailJobFn: func(_ context.Context, _, _ string) error {
			return nil
		},
	}

	processor := NewProcessor(st, blockingHandler{started: started})
	worker := NewWorker(st, processor, WorkerConfig{
		Concurrency:       1,
		PollInterval:      time.Hour,
		LeaseTTL:          200 * time.Millisecond,
		HeartbeatInterval: 20 * time.Millisecond,
		ReclaimInterval:   0,
	}, logger)

	before := time.Now().UTC()

	worker.Start(context.Background())

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Process to start")
	}

	select {
	case leaseUntil := <-heartbeats:
		minUntil := before.Add(200 * time.Millisecond)
		if leaseUntil.Before(minUntil.Add(-50 * time.Millisecond)) {
			t.Errorf("leaseUntil = %v, want around now+TTL", leaseUntil)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for heartbeat")
	}

	worker.Stop(2 * time.Second)
}

func TestWorkerClaimPassesLeaseUntil(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gotLease := make(chan time.Time, 1)

	st := &storetest.Stub{
		ClaimNextPendingJobFn: func(ctx context.Context, leaseUntil time.Time) (*model.Job, error) {
			select {
			case gotLease <- leaseUntil:
			default:
			}

			<-ctx.Done()

			return nil, ctx.Err()
		},
	}

	ttl := 90 * time.Second
	before := time.Now().UTC()

	worker := NewWorker(st, NewProcessor(st), WorkerConfig{
		Concurrency:       1,
		PollInterval:      time.Hour,
		LeaseTTL:          ttl,
		HeartbeatInterval: time.Hour,
		ReclaimInterval:   0,
	}, logger)

	worker.Start(context.Background())

	select {
	case leaseUntil := <-gotLease:
		after := time.Now().UTC()
		wantMin := before.Add(ttl)
		wantMax := after.Add(ttl)

		if leaseUntil.Before(wantMin) || leaseUntil.After(wantMax) {
			t.Errorf("leaseUntil = %v, want between %v and %v", leaseUntil, wantMin, wantMax)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for claim")
	}

	worker.Stop(2 * time.Second)
}

func TestWorkerReclaimLoopCallsReset(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	called := make(chan struct{}, 1)

	st := &storetest.Stub{
		ClaimNextPendingJobFn: func(ctx context.Context, _ time.Time) (*model.Job, error) {
			<-ctx.Done()

			return nil, ctx.Err()
		},
		ResetStaleRunningJobsFn: func(_ context.Context, expiredBefore time.Time) (int64, error) {
			if expiredBefore.IsZero() {
				t.Error("expiredBefore is zero")
			}

			select {
			case called <- struct{}{}:
			default:
			}

			return 0, nil
		},
	}

	worker := NewWorker(st, NewProcessor(st), WorkerConfig{
		Concurrency:     1,
		PollInterval:    time.Hour,
		LeaseTTL:        time.Minute,
		ReclaimInterval: 15 * time.Millisecond,
	}, logger)

	worker.Start(context.Background())

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for periodic reclaim")
	}

	worker.Stop(2 * time.Second)
}
