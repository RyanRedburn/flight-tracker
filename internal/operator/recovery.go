package operator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func RecoverStaleJobs(ctx context.Context, s store.Store, leaseTTL time.Duration, logger *slog.Logger) error {
	if leaseTTL <= 0 {
		return nil
	}

	reset, err := s.ResetStaleRunningJobs(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("reset stale running jobs: %w", err)
	}

	if reset > 0 {
		logger.Info("reclaimed expired job leases", "count", reset, "lease_ttl", leaseTTL)
	}

	return nil
}
