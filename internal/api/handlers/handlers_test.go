package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/operator"
	"github.com/RyanRedburn/flight-tracker/internal/source/mock"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"

	"github.com/go-chi/chi/v5"
)

func newTestJobsHandler(t *testing.T) (*JobsHandler, *mem.Store) {
	t.Helper()

	store := mem.New()
	provider := &mock.Provider{Latency: 0}
	processor := operator.NewProcessor(store, provider)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := operator.NewWorker(processor, 1, logger)
	worker.Start(context.Background())
	t.Cleanup(func() { worker.Stop(2 * time.Second) })

	return NewJobsHandler(store, worker), store
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

func TestJobsCreateAndGet(t *testing.T) {
	h, store := newTestJobsHandler(t)

	body := bytes.NewBufferString(`{"type":"fetch_flights","payload":{"region":"west"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Create status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var created createJobResponse
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	if created.Status != model.JobStatusPending {
		t.Errorf("Status = %q, want pending", created.Status)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", created.ID)
	getReq = getReq.WithContext(context.WithValue(getReq.Context(), chi.RouteCtxKey, rctx))

	getRec := httptest.NewRecorder()
	h.Get(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("Get status = %d, want 200", getRec.Code)
	}

	job, err := store.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if job.Type != "fetch_flights" {
		t.Errorf("Type = %q, want fetch_flights", job.Type)
	}
}

func TestJobsCreateValidation(t *testing.T) {
	h, _ := newTestJobsHandler(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"invalid json", `{`, http.StatusBadRequest},
		{"missing type", `{}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			h.Create(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
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

func TestJobsList(t *testing.T) {
	h, store := newTestJobsHandler(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b"} {
		if err := store.CreateJob(ctx, &model.Job{
			ID:        id,
			Type:      model.JobTypeFetchFlights,
			Status:    model.JobStatusPending,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=1", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var jobs []*model.Job
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
