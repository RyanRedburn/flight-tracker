package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	HTTPAddr                   string        `env:"HTTP_ADDR" envDefault:":8080"`
	DatabaseDriver             string        `env:"DATABASE_DRIVER" envDefault:"postgres"`
	DatabaseURL                string        `env:"DATABASE_URL" envDefault:"postgres://flight:flight@localhost:5432/flight_tracker?sslmode=disable"`
	MigrationsPath             string        `env:"MIGRATIONS_PATH" envDefault:"migrations/postgres"`
	WorkerConcurrency          int           `env:"WORKER_CONCURRENCY" envDefault:"2"`
	WorkerPollInterval         time.Duration `env:"WORKER_POLL_INTERVAL" envDefault:"5s"`
	StaleJobThreshold          time.Duration `env:"STALE_JOB_THRESHOLD" envDefault:"30m"`
	BTSDownloadTimeout         time.Duration `env:"BTS_DOWNLOAD_TIMEOUT" envDefault:"10m"`
	BTSBaseURL                 string        `env:"BTS_BASE_URL" envDefault:"https://transtats.bts.gov/PREZIP"`
	OurAirportsBaseURL         string        `env:"OURAIRPORTS_BASE_URL" envDefault:"https://raw.githubusercontent.com/davidmegginson/ourairports-data/main"`
	OurAirportsDownloadTimeout time.Duration `env:"OURAIRPORTS_DOWNLOAD_TIMEOUT" envDefault:"5m"`
	MaxIngestMonths            int           `env:"MAX_INGEST_MONTHS" envDefault:"24"`
	LogLevel                   slog.Level    `env:"LOG_LEVEL" envDefault:"info"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.ParseWithOptions(&cfg, env.Options{
		Environment: nonEmptyEnv(env.ToMap(os.Environ())),
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeFor[slog.Level](): func(v string) (any, error) {
				return parseLogLevel(v)
			},
			reflect.TypeFor[time.Duration](): func(v string) (any, error) {
				return time.ParseDuration(v)
			},
		},
	}); err != nil {
		return Config{}, err
	}

	if cfg.WorkerConcurrency < 1 {
		return Config{}, errors.New("WORKER_CONCURRENCY must be >= 1")
	}

	if cfg.MaxIngestMonths < 1 {
		return Config{}, errors.New("MAX_INGEST_MONTHS must be >= 1")
	}

	return cfg, nil
}

func nonEmptyEnv(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		if v != "" {
			out[k] = v
		}
	}

	return out
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
