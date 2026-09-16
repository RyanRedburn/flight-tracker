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

	"github.com/RyanRedburn/flight-tracker/internal/api/middleware"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const (
	pathHealth              = "/health"
	pathDBVersion           = "/db/version"
	pathIngestCountries     = "/api/v1/ingest/countries"
	pathCarrierStatsUA      = "/api/v1/carriers/stats?carrier=UA"
	pathSwaggerInternalHTML = "/swagger/internal/index.html"
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
		HasWeatherStationsDataFn: func(context.Context) (bool, error) {
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
			}, nil
		},
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return &model.RouteOutlook{}, nil
		},
		CarrierStatsFn: func(context.Context, store.CarrierStatsFilter) (*model.CarrierStats, error) {
			return &model.CarrierStats{
				BestRoutes:    []model.CarrierRouteStat{},
				WorstRoutes:   []model.CarrierRouteStat{},
				BestAirports:  []model.CarrierAirportStat{},
				WorstAirports: []model.CarrierAirportStat{},
			}, nil
		},
	}
}

func TestNewRouterRoutes(t *testing.T) {
	handler := newRouter(routerStub(), testLogger(), 24, nil, Security{Disabled: true})

	tests := []struct {
		method     string
		path       string
		wantStatus int
	}{
		{http.MethodGet, pathHealth, http.StatusOK},
		{http.MethodGet, "/ready", http.StatusOK},
		{http.MethodGet, pathDBVersion, http.StatusOK},
		{http.MethodGet, "/api/v1/jobs", http.StatusOK},
		{http.MethodPost, pathIngestCountries, http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/regions", http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/airports", http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/weather-stations", http.StatusCreated},
		{http.MethodPost, "/api/v1/ingest/weather", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-01-01&end_date=2026-01-31", http.StatusOK},
		{http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=2&dep_time=0700", http.StatusOK},
		{http.MethodGet, pathCarrierStatsUA, http.StatusOK},
		{http.MethodGet, "/api/v1/flights", http.StatusNotFound},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodGet, "/swagger/index.html", http.StatusOK},
		{http.MethodGet, pathSwaggerInternalHTML, http.StatusOK},
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
	handler := newRouter(routerStub(), testLogger(), 24, nil, Security{Disabled: true})

	external := fetchSwaggerPaths(t, handler, "/swagger/doc.json")
	if _, ok := external["/api/v1/routes/stats"]; !ok {
		t.Fatal("external spec missing /api/v1/routes/stats")
	}

	if _, ok := external["/api/v1/routes/outlook"]; !ok {
		t.Fatal("external spec missing /api/v1/routes/outlook")
	}

	if _, ok := external["/api/v1/carriers/stats"]; !ok {
		t.Fatal("external spec missing /api/v1/carriers/stats")
	}

	if _, ok := external["/api/v1/ingest"]; ok {
		t.Fatal("external spec must not include /api/v1/ingest")
	}

	if _, ok := external["/api/v1/ingest/weather-stations"]; ok {
		t.Fatal("external spec must not include /api/v1/ingest/weather-stations")
	}

	if _, ok := external[pathHealth]; ok {
		t.Fatal("external spec must not include /health")
	}

	if _, ok := external["/api/v1/keys"]; ok {
		t.Fatal("external spec must not include /api/v1/keys")
	}

	internal := fetchSwaggerPaths(t, handler, "/swagger/internal/doc.json")
	for _, path := range []string{
		pathHealth,
		"/api/v1/ingest",
		"/api/v1/ingest/weather-stations",
		"/api/v1/jobs",
		"/api/v1/keys",
		"/api/v1/routes/stats",
		"/api/v1/routes/outlook",
		"/api/v1/carriers/stats",
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
	s := NewServer("unused", routerStub(), testLogger(), 24, nil, Security{Disabled: true})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.httpServer.Serve(ln)
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + pathHealth)
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

func TestNewRouterProbesStayOpenWhenAuthEnabled(t *testing.T) {
	handler := newRouter(routerStub(), testLogger(), 24, nil, Security{})

	req := httptest.NewRequest(http.MethodGet, pathHealth, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestNewRouterAuthEnabled(t *testing.T) {
	adminPlain, adminStub := stubWithAPIKey(t, model.APIKeyRoleAdmin)
	consumerPlain, consumerStub := stubWithAPIKey(t, model.APIKeyRoleConsumer)
	subscriberPlain, subscriberStub := stubWithAPIKey(t, model.APIKeyRoleSubscriber)

	sec := Security{Limits: testRateLimits()}

	tests := []struct {
		name       string
		stub       *storetest.Stub
		method     string
		path       string
		key        string
		wantStatus int
	}{
		{name: "probe unauthenticated", stub: adminStub, method: http.MethodGet, path: pathHealth, wantStatus: http.StatusOK},
		{name: "ready unauthenticated", stub: adminStub, method: http.MethodGet, path: "/ready", wantStatus: http.StatusOK},
		{name: "stats missing key", stub: adminStub, method: http.MethodGet, path: pathCarrierStatsUA, wantStatus: http.StatusUnauthorized},
		{name: "stats consumer", stub: consumerStub, method: http.MethodGet, path: pathCarrierStatsUA, key: consumerPlain, wantStatus: http.StatusOK},
		{name: "stats subscriber", stub: subscriberStub, method: http.MethodGet, path: pathCarrierStatsUA, key: subscriberPlain, wantStatus: http.StatusOK},
		{name: "ingest consumer forbidden", stub: consumerStub, method: http.MethodPost, path: pathIngestCountries, key: consumerPlain, wantStatus: http.StatusForbidden},
		{name: "ingest subscriber forbidden", stub: subscriberStub, method: http.MethodPost, path: pathIngestCountries, key: subscriberPlain, wantStatus: http.StatusForbidden},
		{name: "ingest admin", stub: adminStub, method: http.MethodPost, path: pathIngestCountries, key: adminPlain, wantStatus: http.StatusCreated},
		{name: "db version consumer forbidden", stub: consumerStub, method: http.MethodGet, path: pathDBVersion, key: consumerPlain, wantStatus: http.StatusForbidden},
		{name: "db version admin", stub: adminStub, method: http.MethodGet, path: pathDBVersion, key: adminPlain, wantStatus: http.StatusOK},
		{name: "external swagger consumer", stub: consumerStub, method: http.MethodGet, path: "/swagger/index.html", key: consumerPlain, wantStatus: http.StatusOK},
		{name: "internal swagger consumer forbidden", stub: consumerStub, method: http.MethodGet, path: pathSwaggerInternalHTML, key: consumerPlain, wantStatus: http.StatusForbidden},
		{name: "internal swagger admin", stub: adminStub, method: http.MethodGet, path: pathSwaggerInternalHTML, key: adminPlain, wantStatus: http.StatusOK},
		{name: "keys admin", stub: adminStub, method: http.MethodGet, path: "/api/v1/keys", key: adminPlain, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newRouter(tt.stub, testLogger(), 24, nil, sec)
			req := httptest.NewRequest(tt.method, tt.path, nil)

			if tt.key != "" {
				req.Header.Set("Authorization", "Bearer "+tt.key)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func testRateLimits() middleware.RateLimits {
	return middleware.RateLimits{
		AnonRPM:        60,
		ConsumerRPM:    120,
		SubscriberRPM:  120,
		AdminRPM:       300,
		AdminIngestRPM: 10,
	}
}

func stubWithAPIKey(t *testing.T, role model.APIKeyRole) (string, *storetest.Stub) {
	t.Helper()

	plaintext, prefix, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	key := &model.APIKey{
		ID:        "key-" + string(role),
		Prefix:    prefix,
		KeyHash:   model.HashAPIKey(plaintext),
		Role:      role,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	stub := routerStub()
	stub.LookupAPIKeyByPrefixFn = func(_ context.Context, got string) (*model.APIKey, error) {
		if got == key.Prefix {
			copied := *key
			copied.KeyHash = append([]byte(nil), key.KeyHash...)

			return &copied, nil
		}

		return nil, store.ErrNotFound
	}
	stub.ConsumeRateLimitFn = func(context.Context, string, int) (store.RateLimitResult, error) {
		return store.RateLimitResult{
			Allowed:   true,
			Limit:     300,
			Remaining: 299,
			ResetUnix: time.Now().Add(time.Minute).Unix(),
		}, nil
	}
	stub.ListAPIKeysFn = func(context.Context) ([]*model.APIKey, error) {
		return []*model.APIKey{}, nil
	}

	return plaintext, stub
}
