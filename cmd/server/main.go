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
	btsIngest := bts.NewService(st, btsDownloader)

	processor := operator.NewProcessor(st, operator.NewBTSIngestHandler(st, btsIngest))
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
