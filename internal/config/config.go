package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr          string
	DatabaseDriver    string
	DatabaseURL       string
	MigrationsPath    string
	WorkerConcurrency int
	LogLevel          slog.Level
}

func Load() (Config, error) {
	logLevel, err := parseLogLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}

	workerConcurrency, err := strconv.Atoi(envOr("WORKER_CONCURRENCY", "2"))
	if err != nil {
		return Config{}, fmt.Errorf("WORKER_CONCURRENCY: %w", err)
	}

	if workerConcurrency < 1 {
		return Config{}, errors.New("WORKER_CONCURRENCY must be >= 1")
	}

	return Config{
		HTTPAddr:          envOr("HTTP_ADDR", ":8080"),
		DatabaseDriver:    envOr("DATABASE_DRIVER", "sqlite"),
		DatabaseURL:       envOr("DATABASE_URL", "file:flight-tracker.db"),
		MigrationsPath:    envOr("MIGRATIONS_PATH", "migrations/sqlite"),
		WorkerConcurrency: workerConcurrency,
		LogLevel:          logLevel,
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported LOG_LEVEL %q", raw)
	}
}
