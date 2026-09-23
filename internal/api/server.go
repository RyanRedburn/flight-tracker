package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	_ "github.com/RyanRedburn/flight-tracker/docs/external"
	_ "github.com/RyanRedburn/flight-tracker/docs/full"
	"github.com/RyanRedburn/flight-tracker/internal/api/handlers"
	"github.com/RyanRedburn/flight-tracker/internal/api/middleware"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

type Server struct {
	httpServer *http.Server
}

func newRouter(
	s store.Store,
	logger *slog.Logger,
	maxIngestMonths int,
	weatherStations handlers.WeatherStationResolver,
	sec Security,
) http.Handler {
	health := handlers.NewHealthHandler(s)
	jobs := handlers.NewJobsHandler(s)
	routes := handlers.NewRoutesHandler(s)
	carriers := handlers.NewCarriersHandler(s)
	ingestHandler := handlers.NewIngestHandler(s, maxIngestMonths)
	weatherIngest := handlers.NewWeatherIngestHandler(s, maxIngestMonths, weatherStations, logger)
	referenceIngest := handlers.NewReferenceIngestHandler(s)
	keys := handlers.NewKeysHandler(s)
	protect := middleware.NewProtector(s, middleware.ProtectorConfig{
		Disabled:          sec.Disabled,
		RateLimitDisabled: sec.Disabled || sec.RateLimitDisabled,
		TrustProxy:        sec.TrustProxy,
		Limits:            sec.Limits,
	})

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestLog(logger))

	r.Get("/health", health.Liveness)
	r.Get("/ready", health.Readiness)

	r.Group(func(r chi.Router) {
		r.Use(protect.Require(middleware.AdminOnly, store.RateLimitSurfaceInternal))
		r.Get("/db/version", health.DatabaseVersion)
		// More specific internal UI path before /swagger/*.
		r.Get("/swagger/internal/*", httpSwagger.Handler(
			httpSwagger.InstanceName("internal"),
		))
	})

	r.Group(func(r chi.Router) {
		r.Use(protect.Require(middleware.ExternalRoles, store.RateLimitSurfaceExternal))
		r.Get("/swagger/*", httpSwagger.Handler(
			httpSwagger.InstanceName("external"),
		))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(protect.Require(middleware.AdminOnly, store.RateLimitSurfaceIngest))
			r.Post("/ingest", ingestHandler.Create)
			r.Post("/ingest/weather", weatherIngest.Create)
			r.Post("/ingest/countries", referenceIngest.CreateCountries)
			r.Post("/ingest/regions", referenceIngest.CreateRegions)
			r.Post("/ingest/airports", referenceIngest.CreateAirports)
			r.Post("/ingest/weather-stations", referenceIngest.CreateWeatherStations)
		})

		r.Group(func(r chi.Router) {
			r.Use(protect.Require(middleware.AdminOnly, store.RateLimitSurfaceInternal))
			r.Get("/jobs", jobs.List)
			r.Get("/jobs/{id}", jobs.Get)
			r.Get("/keys", keys.List)
			r.Post("/keys", keys.Create)
			r.Post("/keys/{id}/revoke", keys.Revoke)
		})

		r.Group(func(r chi.Router) {
			r.Use(protect.Require(middleware.ExternalRoles, store.RateLimitSurfaceExternal))
			r.Get("/routes/stats", routes.Stats)
			r.Get("/routes/outlook", routes.Outlook)
			r.Get("/routes/travel-windows", routes.TravelWindows)
			r.Get("/carriers/stats", carriers.Stats)
		})
	})

	return r
}

func NewServer(
	addr string,
	s store.Store,
	logger *slog.Logger,
	maxIngestMonths int,
	weatherStations handlers.WeatherStationResolver,
	sec Security,
) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           newRouter(s, logger, maxIngestMonths, weatherStations, sec),
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
