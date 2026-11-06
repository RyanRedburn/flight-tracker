package postgres

import (
	"context"
	"fmt"
)

func (s *Store) rebuildRollup(ctx context.Context, lockKey, truncateQuery, insertQuery, truncateErr, insertErr string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, QueryAdvisoryXactLock, lockKey, 0); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}

	if _, err := tx.ExecContext(ctx, truncateQuery); err != nil {
		return fmt.Errorf("%s: %w", truncateErr, err)
	}

	if _, err := tx.ExecContext(ctx, insertQuery); err != nil {
		return fmt.Errorf("%s: %w", insertErr, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
