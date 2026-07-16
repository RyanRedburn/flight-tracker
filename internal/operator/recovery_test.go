package operator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestRecoverStaleJobs(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var gotCutoff time.Time
	var called bool

	st := &storetest.Stub{
		ResetStaleRunningJobsFn: func(_ context.Context, olderThan time.Time) (int64, error) {
			called = true
			gotCutoff = olderThan

			return 1, nil
		},
	}

	before := time.Now().UTC()
	if err := RecoverStaleJobs(ctx, st, time.Hour, logger); err != nil {
		t.Fatalf("RecoverStaleJobs() error = %v", err)
	}
	after := time.Now().UTC()

	if !called {
		t.Fatal("expected ResetStaleRunningJobs to be called")
	}

	wantMin := before.Add(-time.Hour)
	wantMax := after.Add(-time.Hour)

	if gotCutoff.Before(wantMin) || gotCutoff.After(wantMax) {
		t.Errorf("olderThan = %v, want between %v and %v", gotCutoff, wantMin, wantMax)
	}
}

func TestRecoverStaleJobsZeroThreshold(t *testing.T) {
	st := &storetest.Stub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := RecoverStaleJobs(context.Background(), st, 0, logger); err != nil {
		t.Fatalf("RecoverStaleJobs() error = %v", err)
	}
}
