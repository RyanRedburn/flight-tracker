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

func TestJobsGetEnrichedFlightPerformanceIngest(t *testing.T) {
	const jobID = "job-bts-1"

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	h := NewJobsHandler(&storetest.Stub{
		GetJobFn: func(_ context.Context, id string) (*model.Job, error) {
			return &model.Job{
				ID:        id,
				Type:      model.JobTypeImportFlightPerformance,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		GetFlightPerformanceIngestJobFn: func(_ context.Context, id string) (*model.FlightPerformanceIngestJob, error) {
			return &model.FlightPerformanceIngestJob{JobID: id, Year: 2026, Month: 4}, nil
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

func TestJobsGetEnrichedWeatherIngest(t *testing.T) {
	const jobID = "job-weather-1"

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	h := NewJobsHandler(&storetest.Stub{
		GetJobFn: func(_ context.Context, id string) (*model.Job, error) {
			return &model.Job{
				ID:        id,
				Type:      model.JobTypeImportWeatherObservations,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		GetWeatherIngestJobFn: func(_ context.Context, id string) (*model.WeatherIngestJob, error) {
			return &model.WeatherIngestJob{
				JobID:    id,
				Year:     2024,
				Month:    1,
				Stations: []string{"ORD", "JFK"},
			}, nil
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

	if body.Year == nil || *body.Year != 2024 {
		t.Errorf("year = %v, want 2024", body.Year)
	}

	if body.Month == nil || *body.Month != 1 {
		t.Errorf("month = %v, want 1", body.Month)
	}

	if len(body.Stations) != 2 || body.Stations[0] != "ORD" || body.Stations[1] != "JFK" {
		t.Errorf("stations = %v, want [ORD JFK]", body.Stations)
	}
}

func TestJobsGetUnknownType(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	h := NewJobsHandler(&storetest.Stub{
		GetJobFn: func(_ context.Context, id string) (*model.Job, error) {
			return &model.Job{
				ID:        id,
				Type:      model.JobType("not_a_job"),
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-unknown", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "job-unknown")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestJobsList(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	flightYear, flightMonth := 2026, 4
	weatherYear, weatherMonth := 2024, 1

	var flightDetailGets, weatherDetailGets int

	h := NewJobsHandler(&storetest.Stub{
		ListJobsFn: func(_ context.Context, limit int) ([]*model.Job, error) {
			if limit != 2 {
				t.Errorf("limit = %d, want 2", limit)
			}

			return []*model.Job{
				{
					ID:        testJobID,
					Type:      model.JobTypeImportFlightPerformance,
					Status:    model.JobStatusPending,
					CreatedAt: now,
					UpdatedAt: now,
					ListIngest: &model.JobListIngest{
						Year:  &flightYear,
						Month: &flightMonth,
					},
				},
				{
					ID:        "job-weather-1",
					Type:      model.JobTypeImportWeatherObservations,
					Status:    model.JobStatusPending,
					CreatedAt: now,
					UpdatedAt: now,
					ListIngest: &model.JobListIngest{
						Year:     &weatherYear,
						Month:    &weatherMonth,
						Stations: []string{testOriginORD, testDestJFK},
					},
				},
			}, nil
		},
		GetFlightPerformanceIngestJobFn: func(context.Context, string) (*model.FlightPerformanceIngestJob, error) {
			flightDetailGets++

			return nil, errors.New("unexpected flight detail lookup")
		},
		GetWeatherIngestJobFn: func(context.Context, string) (*model.WeatherIngestJob, error) {
			weatherDetailGets++

			return nil, errors.New("unexpected weather detail lookup")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=2", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if flightDetailGets != 0 || weatherDetailGets != 0 {
		t.Errorf("detail getters flight=%d weather=%d, want 0", flightDetailGets, weatherDetailGets)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var jobs []JobResponse
	if err := json.NewDecoder(rec.Body).Decode(&jobs); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}

	if jobs[0].Year == nil || *jobs[0].Year != flightYear || jobs[0].Month == nil || *jobs[0].Month != flightMonth {
		t.Errorf("flight year/month = %v/%v, want %d/%d", jobs[0].Year, jobs[0].Month, flightYear, flightMonth)
	}

	if len(jobs[0].Stations) != 0 {
		t.Errorf("flight stations = %v, want none", jobs[0].Stations)
	}

	if jobs[1].Year == nil || *jobs[1].Year != weatherYear || jobs[1].Month == nil || *jobs[1].Month != weatherMonth {
		t.Errorf("weather year/month = %v/%v, want %d/%d", jobs[1].Year, jobs[1].Month, weatherYear, weatherMonth)
	}

	if len(jobs[1].Stations) != 2 || jobs[1].Stations[0] != testOriginORD || jobs[1].Stations[1] != testDestJFK {
		t.Errorf("weather stations = %v, want [%s %s]", jobs[1].Stations, testOriginORD, testDestJFK)
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
