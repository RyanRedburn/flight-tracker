package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"

	"github.com/go-chi/chi/v5"
)

const (
	testFlightDate20260401 = "2026-04-01"
	testFlightDate20260424 = "2026-04-24"
	testAirportORD         = "ORD"
	testAirportBHM         = "BHM"
	testAirportLAX         = "LAX"
	testFloatNo            = "0.00"
	testFloatYes           = "1.00"
)

func newTestJobsHandler(t *testing.T) (*JobsHandler, *mem.Store) {
	t.Helper()

	store := mem.New()

	return NewJobsHandler(store), store
}

func TestHealthLiveness(t *testing.T) {
	h := NewHealthHandler(mem.New())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestHealthReadiness(t *testing.T) {
	store := mem.New()
	h := NewHealthHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	pingErr := errors.New("db unavailable")

	store.SetPingHook(func(context.Context) error { return pingErr })

	rec = httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHealthDatabaseVersion(t *testing.T) {
	h := NewHealthHandler(mem.New())

	req := httptest.NewRequest(http.MethodGet, "/db/version", nil)
	rec := httptest.NewRecorder()
	h.DatabaseVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body store.MigrationVersion
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Version != 0 || body.Dirty {
		t.Errorf("version = %+v, want {Version:0 Dirty:false}", body)
	}
}

func TestJobsGetNotFound(t *testing.T) {
	h, _ := newTestJobsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/missing", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestJobsGetEnrichedBTSIngest(t *testing.T) {
	h, store := newTestJobsHandler(t)
	ctx := context.Background()

	job, err := store.CreateBTSIngestJob(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+job.ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", job.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body jobResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Year == nil || *body.Year != 2026 {
		t.Errorf("year = %v, want 2026", body.Year)
	}

	if body.Month == nil || *body.Month != 4 {
		t.Errorf("month = %v, want 4", body.Month)
	}
}

func TestJobsList(t *testing.T) {
	h, store := newTestJobsHandler(t)
	ctx := context.Background()

	for range 2 {
		if _, err := store.CreateBTSIngestJob(ctx, 2026, 4); err != nil {
			t.Fatalf("CreateBTSIngestJob() error = %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=1", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var jobs []jobResponse
	if err := json.NewDecoder(rec.Body).Decode(&jobs); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
}

func TestJobsListInvalidLimit(t *testing.T) {
	h, _ := newTestJobsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=0", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
