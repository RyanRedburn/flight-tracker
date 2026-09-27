// Package main is the flight-tracker HTTP server entrypoint.
//
//	@title						flight-tracker API
//	@version					1.0
//	@description				REST API for flight data ingest, job status, route performance, and carrier performance. Protected routes require an API key (`Authorization: Bearer <key>` or `X-API-Key`). Rate-limited responses return 429 with `Retry-After`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset`.
//	@host						localhost:8080
//	@BasePath					/
//
//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						Authorization
//	@description				API key as `Bearer <key>`. `X-API-Key` is also accepted.
//
//	@tag.name					health
//	@tag.description			Liveness, readiness, and database migration version
//	@tag.name					ingest
//	@tag.description			Queue flight performance, weather, and reference data import jobs
//	@tag.name					jobs
//	@tag.description			Inspect background job status
//	@tag.name					freshness
//	@tag.description			Admin dataset freshness (last successful ingest and latest covered period)
//	@tag.name					rebuild
//	@tag.description			Queue a full rebuild of travel-window rollups
//	@tag.name					routes
//	@tag.description			Route performance stats, booking outlook, and typical-year travel windows
//	@tag.name					carriers
//	@tag.description			Carrier performance stats
//	@tag.name					keys
//	@tag.description			Admin API key management
//
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d .,../../internal/api -o ../../docs/external --instanceName external --tags external --parseDependency --parseInternal
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d .,../../internal/api -o ../../docs/full --instanceName internal --parseDependency --parseInternal
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/api"
	"github.com/RyanRedburn/flight-tracker/internal/api/middleware"
	"github.com/RyanRedburn/flight-tracker/internal/config"
	"github.com/RyanRedburn/flight-tracker/internal/database"
	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/operator"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx := context.Background()

	st, err := database.NewStore(ctx, cfg)
	if err != nil {
		logger.Error("open store", "error", err)
		return 1
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("close store", "error", err)
		}
	}()

	if err := operator.RecoverStaleJobs(ctx, st, cfg.JobLeaseTTL, logger); err != nil {
		logger.Error("recover stale jobs", "error", err)
		return 1
	}

	created, prefix, err := api.BootstrapAdminKey(ctx, st, cfg.AuthBootstrapAdminKey)
	if err != nil {
		logger.Error("bootstrap admin API key", "error", err)
		return 1
	}

	if created {
		logger.Info("bootstrapped admin API key", "prefix", prefix)
	}

	if err := api.RequireAPIKeysIfAuthEnabled(ctx, st, cfg.AuthDisabled); err != nil {
		logger.Error("auth startup check", "error", err)
		return 1
	}

	if cfg.AuthDisabled {
		logger.Warn("API authentication and rate limits are disabled (AUTH_DISABLED=true); do not use this in production")
	}

	btsDownloader := bts.NewDownloader(cfg.BTSBaseURL, cfg.BTSDownloadTimeout)
	flightPerformanceIngest := bts.NewService(st, btsDownloader)

	iemDownloader := iem.NewDownloader(cfg.IEMASOSBaseURL, cfg.IEMASOSDownloadTimeout)
	iemCatalog := iem.NewNetworkCatalog(cfg.IEMGeoJSONBaseURL, cfg.IEMGeoJSONTimeout)
	weatherIngest := iem.NewService(st, iemDownloader).WithCatalog(iemCatalog)
	weatherStations := iem.NewStationResolver(st, logger)

	oaDownloader := ourairports.NewDownloader(cfg.OurAirportsBaseURL, cfg.OurAirportsDownloadTimeout)
	oaIngest := ourairports.NewService(st, oaDownloader)

	processor, err := operator.NewProcessor(st,
		operator.NewFlightPerformanceIngestHandler(st, flightPerformanceIngest),
		operator.NewRebuildRouteTravelWindowsHandler(st),
		operator.NewRebuildRouteWeatherStatsHandler(st),
		operator.NewWeatherIngestHandler(st, weatherIngest),
		operator.NewWeatherStationsHandler(st, weatherIngest),
		operator.NewCountriesHandler(oaIngest),
		operator.NewRegionsHandler(oaIngest),
		operator.NewAirportsHandler(oaIngest),
	)
	if err != nil {
		logger.Error("build job processor", "error", err)
		return 1
	}

	worker := operator.NewWorker(st, processor, operator.WorkerConfig{
		Concurrency:  cfg.WorkerConcurrency,
		PollInterval: cfg.WorkerPollInterval,
		LeaseTTL:     cfg.JobLeaseTTL,
	}, logger)

	worker.Start(ctx)
	defer worker.Stop(15 * time.Second)

	server := api.NewServer(cfg.HTTPAddr, st, logger, cfg.MaxIngestMonths, weatherStations, api.Security{
		Disabled:          cfg.AuthDisabled,
		RateLimitDisabled: cfg.RateLimitDisabled,
		TrustProxy:        cfg.RateLimitTrustProxy,
		Limits: middleware.RateLimits{
			AnonRPM:        cfg.RateLimitAnonRPM,
			ConsumerRPM:    cfg.RateLimitConsumerRPM,
			SubscriberRPM:  cfg.RateLimitSubscriberRPM,
			AdminRPM:       cfg.RateLimitAdminRPM,
			AdminIngestRPM: cfg.RateLimitAdminIngestRPM,
			AuthFailRPM:    cfg.RateLimitAuthFailRPM,
		},
	})

	serverErr := make(chan error, 1)

	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
	case err := <-serverErr:
		logger.Error("http server", "error", err)
		return 1
	}

	logger.Info("shutting down")

	worker.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", "error", err)
		return 1
	}

	return 0
}
