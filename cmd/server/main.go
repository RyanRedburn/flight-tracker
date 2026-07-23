// Package main is the flight-tracker HTTP server entrypoint.
//
//	@title						flight-tracker API
//	@version					1.0
//	@description				REST API for flight data ingest, job status, and route performance.
//	@host						localhost:8080
//	@BasePath					/
//
//	@tag.name					health
//	@tag.description			Liveness, readiness, and database migration version
//	@tag.name					ingest
//	@tag.description			Queue flight performance, weather, and reference data import jobs
//	@tag.name					jobs
//	@tag.description			Inspect background job status
//	@tag.name					routes
//	@tag.description			Route performance stats and booking outlook
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

	if err := operator.RecoverStaleJobs(ctx, st, cfg.StaleJobThreshold, logger); err != nil {
		logger.Error("recover stale jobs", "error", err)
		return 1
	}

	btsDownloader := bts.NewDownloader(cfg.BTSBaseURL, cfg.BTSDownloadTimeout)
	flightPerformanceIngest := bts.NewService(st, btsDownloader)

	iemDownloader := iem.NewDownloader(cfg.IEMASOSBaseURL, cfg.IEMASOSDownloadTimeout)
	weatherIngest := iem.NewService(st, iemDownloader)

	oaDownloader := ourairports.NewDownloader(cfg.OurAirportsBaseURL, cfg.OurAirportsDownloadTimeout)
	oaIngest := ourairports.NewService(st, oaDownloader)

	processor := operator.NewProcessor(st,
		operator.NewFlightPerformanceIngestHandler(st, flightPerformanceIngest),
		operator.NewWeatherIngestHandler(st, weatherIngest),
		operator.NewCountriesHandler(oaIngest),
		operator.NewRegionsHandler(oaIngest),
		operator.NewAirportsHandler(oaIngest),
	)
	worker := operator.NewWorker(st, processor, cfg.WorkerConcurrency, cfg.WorkerPollInterval, logger)

	worker.Start(ctx)
	defer worker.Stop(10 * time.Second)

	server := api.NewServer(cfg.HTTPAddr, st, logger, cfg.MaxIngestMonths)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", "error", err)
		return 1
	}

	return 0
}
