package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// routerStub returns success for routes exercised by router smoke tests.
func routerStub() *storetest.Stub {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	return &storetest.Stub{
		PingFn: func(context.Context) error { return nil },
		MigrationVersionFn: func(context.Context) (store.MigrationVersion, error) {
			return store.MigrationVersion{}, nil
		},
		ListJobsFn: func(context.Context, int) ([]*model.Job, error) {
			return []*model.Job{}, nil
		},
		ActiveIngestJobFn: func(context.Context, string) (bool, error) {
			return false, nil
		},
		HasReferenceDataFn: func(context.Context, store.ReferenceDataset) (bool, error) {
			return false, nil
		},
		CreateReferenceIngestJobFn: func(_ context.Context, jobType string) (*model.Job, error) {
			return &model.Job{
				ID:        "job-oa",
				Type:      jobType,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				DiversionAirports: []model.AirportCount{},
				CancellationCodes: []model.CancellationCodeCount{},
			}, nil
		},
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return &model.RouteOutlook{}, nil
		},
	}
}

func TestNewRouterRoutes(t *testing.T) {
	handler := newRouter(routerStub(), testLogger(), 24)

	tests := []struct {
		method     string
		path       string
		wantStatus int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/ready", http.StatusOK},
		{http.MethodGet, "/db/version", http.StatusOK},
		{http.MethodGet, "/api/v1/jobs", http.StatusOK},
		{http.MethodPost, "/api/v1/ingest/countries", http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/regions", http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/airports", http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/weather", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-01-01&end_date=2026-01-31", http.StatusOK},
		{http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700", http.StatusOK},
		{http.MethodGet, "/api/v1/flights", http.StatusNotFound},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodGet, "/swagger/index.html", http.StatusOK},
		{http.MethodGet, "/swagger/internal/index.html", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestSwaggerSpecSurfaces(t *testing.T) {
	handler := newRouter(routerStub(), testLogger(), 24)

	external := fetchSwaggerPaths(t, handler, "/swagger/doc.json")
	if _, ok := external["/api/v1/routes/stats"]; !ok {
		t.Fatal("external spec missing /api/v1/routes/stats")
	}

	if _, ok := external["/api/v1/routes/outlook"]; !ok {
		t.Fatal("external spec missing /api/v1/routes/outlook")
	}

	if _, ok := external["/api/v1/ingest"]; ok {
		t.Fatal("external spec must not include /api/v1/ingest")
	}

	if _, ok := external["/health"]; ok {
		t.Fatal("external spec must not include /health")
	}

	internal := fetchSwaggerPaths(t, handler, "/swagger/internal/doc.json")
	for _, path := range []string{
		"/health",
		"/api/v1/ingest",
		"/api/v1/jobs",
		"/api/v1/routes/stats",
		"/api/v1/routes/outlook",
	} {
		if _, ok := internal[path]; !ok {
			t.Fatalf("internal spec missing %s", path)
		}
	}
}

func fetchSwaggerPaths(t *testing.T, handler http.Handler, path string) map[string]json.RawMessage {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d; body=%s", path, rec.Code, http.StatusOK, rec.Body.String())
	}

	var spec struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	if spec.Paths == nil {
		t.Fatalf("%s has nil paths", path)
	}

	return spec.Paths
}

func TestServerShutdown(t *testing.T) {
	s := NewServer("unused", routerStub(), testLogger(), 24)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.httpServer.Serve(ln)
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve() error = %v", err)
	}
}
