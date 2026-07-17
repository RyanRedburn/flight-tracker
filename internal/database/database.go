package database

import (
	"context"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/config"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/postgres"
)

func NewStore(ctx context.Context, cfg config.Config) (store.Store, error) {
	switch cfg.DatabaseDriver {
	case "postgres":
		return postgres.Open(ctx, cfg.DatabaseURL, cfg.MigrationsPath)
	default:
		return nil, fmt.Errorf("unsupported DATABASE_DRIVER: %q", cfg.DatabaseDriver)
	}
}
