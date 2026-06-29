package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/config"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/sqlite"
)

var errPostgresNotImplemented = errors.New("postgres store not implemented yet")

func NewStore(ctx context.Context, cfg config.Config) (store.Store, error) {
	switch cfg.DatabaseDriver {
	case "sqlite":
		return sqlite.Open(ctx, cfg.DatabaseURL, cfg.MigrationsPath)
	case "postgres":
		return nil, errPostgresNotImplemented
	default:
		return nil, fmt.Errorf("unsupported DATABASE_DRIVER: %q", cfg.DatabaseDriver)
	}
}
