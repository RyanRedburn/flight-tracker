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
	t.Setenv("BTS_DOWNLOAD_TIMEOUT", "")
	t.Setenv("BTS_BASE_URL", "")
	t.Setenv("IEM_ASOS_BASE_URL", "")
	t.Setenv("IEM_ASOS_DOWNLOAD_TIMEOUT", "")
	t.Setenv("IEM_GEOJSON_BASE_URL", "")
	t.Setenv("IEM_GEOJSON_TIMEOUT", "")
	t.Setenv("OURAIRPORTS_BASE_URL", "")
	t.Setenv("OURAIRPORTS_DOWNLOAD_TIMEOUT", "")
	t.Setenv("MAX_INGEST_MONTHS", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}

	if cfg.DatabaseDriver != "postgres" {
		t.Errorf("DatabaseDriver = %q, want postgres", cfg.DatabaseDriver)
	}

	wantURL := "postgres://flight:flight@localhost:5432/flight_tracker?sslmode=disable" //nolint:gosec // G101: local-dev default DSN
	if cfg.DatabaseURL != wantURL {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, wantURL)
	}

	if cfg.MigrationsPath != "migrations/postgres" {
		t.Errorf("MigrationsPath = %q, want migrations/postgres", cfg.MigrationsPath)
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

	if cfg.BTSDownloadTimeout != 10*time.Minute {
		t.Errorf("BTSDownloadTimeout = %v, want 10m", cfg.BTSDownloadTimeout)
	}

	if cfg.BTSBaseURL != "https://transtats.bts.gov/PREZIP" {
		t.Errorf("BTSBaseURL = %q, want default transtats URL", cfg.BTSBaseURL)
	}

	if cfg.IEMASOSBaseURL != "https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py" {
		t.Errorf("IEMASOSBaseURL = %q, want default IEM asos.py URL", cfg.IEMASOSBaseURL)
	}

	if cfg.IEMASOSDownloadTimeout != 10*time.Minute {
		t.Errorf("IEMASOSDownloadTimeout = %v, want 10m", cfg.IEMASOSDownloadTimeout)
	}

	if cfg.IEMGeoJSONBaseURL != "https://mesonet.agron.iastate.edu/geojson/network" {
		t.Errorf("IEMGeoJSONBaseURL = %q, want default IEM geojson URL", cfg.IEMGeoJSONBaseURL)
	}

	if cfg.IEMGeoJSONTimeout != 2*time.Minute {
		t.Errorf("IEMGeoJSONTimeout = %v, want 2m", cfg.IEMGeoJSONTimeout)
	}

	if cfg.OurAirportsBaseURL != "https://raw.githubusercontent.com/davidmegginson/ourairports-data/main" {
		t.Errorf("OurAirportsBaseURL = %q, want default ourairports URL", cfg.OurAirportsBaseURL)
	}

	if cfg.OurAirportsDownloadTimeout != 5*time.Minute {
		t.Errorf("OurAirportsDownloadTimeout = %v, want 5m", cfg.OurAirportsDownloadTimeout)
	}

	if cfg.MaxIngestMonths != 24 {
		t.Errorf("MaxIngestMonths = %d, want 24", cfg.MaxIngestMonths)
	}

	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://flight:flight@localhost:5432/flight_tracker?sslmode=disable")
	t.Setenv("MIGRATIONS_PATH", "migrations/postgres")
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

func TestLoadInvalidMaxIngestMonths(t *testing.T) {
	t.Setenv("MAX_INGEST_MONTHS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for MAX_INGEST_MONTHS=0")
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
