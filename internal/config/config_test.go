package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MIGRATIONS_PATH", "")
	t.Setenv("WORKER_CONCURRENCY", "")
	t.Setenv("WORKER_POLL_INTERVAL", "")
	t.Setenv("STALE_JOB_THRESHOLD", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}

	if cfg.DatabaseDriver != "sqlite" {
		t.Errorf("DatabaseDriver = %q, want sqlite", cfg.DatabaseDriver)
	}

	if cfg.DatabaseURL != "file:flight-tracker.db" {
		t.Errorf("DatabaseURL = %q, want file:flight-tracker.db", cfg.DatabaseURL)
	}

	if cfg.MigrationsPath != "migrations/sqlite" {
		t.Errorf("MigrationsPath = %q, want migrations/sqlite", cfg.MigrationsPath)
	}

	if cfg.WorkerConcurrency != 2 {
		t.Errorf("WorkerConcurrency = %d, want 2", cfg.WorkerConcurrency)
	}

	if cfg.WorkerPollInterval != 5*time.Second {
		t.Errorf("WorkerPollInterval = %v, want 5s", cfg.WorkerPollInterval)
	}

	if cfg.StaleJobThreshold != 30*time.Minute {
		t.Errorf("StaleJobThreshold = %v, want 30m", cfg.StaleJobThreshold)
	}

	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", "file:/tmp/test.db")
	t.Setenv("MIGRATIONS_PATH", "migrations/sqlite")
	t.Setenv("WORKER_CONCURRENCY", "4")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}

	if cfg.WorkerConcurrency != 4 {
		t.Errorf("WorkerConcurrency = %d, want 4", cfg.WorkerConcurrency)
	}

	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

func TestLoadInvalidWorkerConcurrency(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for WORKER_CONCURRENCY=0")
	}
}

func TestLoadInvalidWorkerConcurrencyValue(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid WORKER_CONCURRENCY")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if err != nil {
				t.Fatalf("parseLogLevel(%q) error = %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseLogLevelInvalid(t *testing.T) {
	_, err := parseLogLevel("verbose")
	if err == nil {
		t.Fatal("parseLogLevel() expected error for unsupported level")
	}
}
