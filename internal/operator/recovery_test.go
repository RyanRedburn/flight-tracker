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
		ResetStaleRunningJobsFn: func(_ context.Context, expiredBefore time.Time) (int64, error) {
			called = true
			gotCutoff = expiredBefore

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

	if gotCutoff.Before(before) || gotCutoff.After(after) {
		t.Errorf("expiredBefore = %v, want between %v and %v", gotCutoff, before, after)
	}
}

func TestRecoverStaleJobsZeroTTL(t *testing.T) {
	st := &storetest.Stub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := RecoverStaleJobs(context.Background(), st, 0, logger); err != nil {
		t.Fatalf("RecoverStaleJobs() error = %v", err)
	}
}
