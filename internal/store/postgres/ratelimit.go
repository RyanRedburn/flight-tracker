package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const staleRateLimitTTL = 2 * time.Hour

func (s *Store) ConsumeRateLimit(ctx context.Context, bucketKey string, requestsPerMinute int) (store.RateLimitResult, error) {
	if requestsPerMinute < 1 {
		return store.RateLimitResult{}, errors.New("requests per minute must be >= 1")
	}

	var (
		allowed bool
		tokens  int64
		now     time.Time
	)

	err := s.db.QueryRowContext(ctx, store.QueryConsumeRateLimit, bucketKey, requestsPerMinute).Scan(
		&allowed,
		&tokens,
		&now,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.RateLimitResult{}, errors.New("rate limit bucket missing after consume")
	}

	if err != nil {
		return store.RateLimitResult{}, fmt.Errorf("consume rate limit: %w", err)
	}

	return store.RateLimitResultFromMilli(allowed, int(tokens), requestsPerMinute, now.UTC()), nil
}

func (s *Store) DeleteStaleRateLimitBuckets(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, store.QueryDeleteStaleRateLimitBuckets, time.Now().UTC().Add(-staleRateLimitTTL)); err != nil {
		return fmt.Errorf("delete stale rate limit buckets: %w", err)
	}

	return nil
}
