package operator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func RecoverStaleJobs(ctx context.Context, s store.Store, threshold time.Duration, logger *slog.Logger) error {
	if threshold <= 0 {
		return nil
	}

	cutoff := time.Now().UTC().Add(-threshold)

	reset, err := s.ResetStaleRunningJobs(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("reset stale running jobs: %w", err)
	}

	if reset > 0 {
		logger.Info("reset stale running jobs", "count", reset, "threshold", threshold)
	}

	return nil
}
