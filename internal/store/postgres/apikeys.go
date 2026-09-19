package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolation = "23505"

func (s *Store) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	var revokedAt sql.NullTime
	if key.RevokedAt != nil {
		revokedAt = sql.NullTime{Time: key.RevokedAt.UTC(), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, store.QueryCreateAPIKey,
		key.ID,
		key.Prefix,
		key.KeyHash,
		string(key.Role),
		key.Name,
		key.CreatedAt.UTC(),
		revokedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return store.ErrConflict
		}

		return fmt.Errorf("create api key: %w", err)
	}

	return nil
}

func (s *Store) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*model.APIKey, error) {
	key, err := scanAPIKey(s.db.QueryRowContext(ctx, store.QueryLookupAPIKeyByPrefix, prefix))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("api key prefix %q: %w", prefix, store.ErrNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("lookup api key: %w", err)
	}

	return key, nil
}

func (s *Store) GetAPIKey(ctx context.Context, id string) (*model.APIKey, error) {
	key, err := scanAPIKey(s.db.QueryRowContext(ctx, store.QueryGetAPIKey, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("api key %q: %w", id, store.ErrNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}

	return key, nil
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]*model.APIKey, error) {
	rows, err := s.db.QueryContext(ctx, store.QueryListAPIKeys)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	keys := make([]*model.APIKey, 0)

	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

func (s *Store) RevokeAPIKey(ctx context.Context, id string, revokedAt time.Time) error {
	res, err := s.db.ExecContext(ctx, store.QueryRevokeAPIKey, revokedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api key rows: %w", err)
	}

	if n > 0 {
		return nil
	}

	if _, err := s.GetAPIKey(ctx, id); err != nil {
		return err
	}

	return nil
}

func (s *Store) CountAPIKeys(ctx context.Context) (int64, error) {
	var n int64
	if err := s.db.QueryRowContext(ctx, store.QueryCountAPIKeys).Scan(&n); err != nil {
		return 0, fmt.Errorf("count api keys: %w", err)
	}

	return n, nil
}

func scanAPIKey(row rowScanner) (*model.APIKey, error) {
	var (
		key       model.APIKey
		role      string
		createdAt time.Time
		revokedAt sql.NullTime
	)

	if err := row.Scan(
		&key.ID,
		&key.Prefix,
		&key.KeyHash,
		&role,
		&key.Name,
		&createdAt,
		&revokedAt,
	); err != nil {
		return nil, err
	}

	key.Role = model.APIKeyRole(role)
	key.CreatedAt = createdAt.UTC()

	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		key.RevokedAt = &t
	}

	return &key, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}

	return false
}
