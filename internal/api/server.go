package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/api/handlers"
	"github.com/RyanRedburn/flight-tracker/internal/api/middleware"
	"github.com/RyanRedburn/flight-tracker/internal/operator"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(addr string, s store.Store, worker *operator.Worker, logger *slog.Logger) *Server {
	health := handlers.NewHealthHandler(s)
	jobs := handlers.NewJobsHandler(s, worker)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestLog(logger))

	r.Get("/health", health.Liveness)
	r.Get("/ready", health.Readiness)
	r.Get("/db/version", health.DatabaseVersion)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/jobs", jobs.Create)
		r.Get("/jobs", jobs.List)
		r.Get("/jobs/{id}", jobs.Get)
	})

	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
