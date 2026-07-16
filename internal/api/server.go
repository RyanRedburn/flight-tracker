package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/api/handlers"
	"github.com/RyanRedburn/flight-tracker/internal/api/middleware"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	httpServer *http.Server
}

func newRouter(s store.Store, logger *slog.Logger, maxIngestMonths int) http.Handler {
	health := handlers.NewHealthHandler(s)
	jobs := handlers.NewJobsHandler(s)
	routes := handlers.NewRoutesHandler(s)
	ingestHandler := handlers.NewIngestHandler(s, maxIngestMonths)
	oaIngest := handlers.NewOurAirportsIngestHandler(s)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestLog(logger))

	r.Get("/health", health.Liveness)
	r.Get("/ready", health.Readiness)
	r.Get("/db/version", health.DatabaseVersion)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/ingest", ingestHandler.Create)
		r.Post("/ingest/countries", oaIngest.CreateCountries)
		r.Post("/ingest/regions", oaIngest.CreateRegions)
		r.Post("/ingest/airports", oaIngest.CreateAirports)
		r.Get("/jobs", jobs.List)
		r.Get("/jobs/{id}", jobs.Get)
		r.Get("/routes/stats", routes.Stats)
		r.Get("/routes/outlook", routes.Outlook)
	})

	return r
}

func NewServer(addr string, s store.Store, logger *slog.Logger, maxIngestMonths int) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           newRouter(s, logger, maxIngestMonths),
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
