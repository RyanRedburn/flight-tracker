package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"

	"github.com/go-chi/chi/v5"
)

func TestHealthLiveness(t *testing.T) {
	// Liveness does not call the store.
	h := NewHealthHandler(&storetest.Stub{})

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
	st := &storetest.Stub{
		PingFn: func(context.Context) error { return nil },
	}
	h := NewHealthHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	st.PingFn = func(context.Context) error { return errors.New("db unavailable") }

	rec = httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHealthDatabaseVersion(t *testing.T) {
	h := NewHealthHandler(&storetest.Stub{
		MigrationVersionFn: func(context.Context) (store.MigrationVersion, error) {
			return store.MigrationVersion{}, nil
		},
	})

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
	h := NewJobsHandler(&storetest.Stub{
		GetJobFn: func(context.Context, string) (*model.Job, error) {
			return nil, store.ErrNotFound
		},
	})

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
	const jobID = "job-bts-1"

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	h := NewJobsHandler(&storetest.Stub{
		GetJobFn: func(_ context.Context, id string) (*model.Job, error) {
			return &model.Job{
				ID:        id,
				Type:      model.JobTypeImportBTSOnTime,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		GetBTSIngestJobFn: func(_ context.Context, id string) (*model.BTSIngestJob, error) {
			return &model.BTSIngestJob{JobID: id, Year: 2026, Month: 4}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", jobID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body JobResponse
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
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	h := NewJobsHandler(&storetest.Stub{
		ListJobsFn: func(context.Context, int) ([]*model.Job, error) {
			// Scenario: store already returns the limited page.
			return []*model.Job{
				{
					ID:        "job-1",
					Type:      model.JobTypeImportBTSOnTime,
					Status:    model.JobStatusPending,
					CreatedAt: now,
					UpdatedAt: now,
				},
			}, nil
		},
		GetBTSIngestJobFn: func(_ context.Context, jobID string) (*model.BTSIngestJob, error) {
			return &model.BTSIngestJob{JobID: jobID, Year: 2026, Month: 4}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=1", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var jobs []JobResponse
	if err := json.NewDecoder(rec.Body).Decode(&jobs); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
}

func TestJobsListInvalidLimit(t *testing.T) {
	// Invalid limit is rejected before any store call.
	h := NewJobsHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=0", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
