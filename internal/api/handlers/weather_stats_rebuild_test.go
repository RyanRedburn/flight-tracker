package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestRebuildWeatherStatsCreate(t *testing.T) {
	tests := []struct {
		name string
		body *bytes.Reader
	}{
		{name: "empty", body: bytes.NewReader(nil)},
		{name: "empty object", body: bytes.NewReader([]byte("{}"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var checkedType model.JobType

			h := NewRebuildWeatherStatsHandler(&storetest.Stub{
				ActiveIngestJobFn: func(_ context.Context, jobType model.JobType) (bool, error) {
					checkedType = jobType

					return false, nil
				},
				CreateRebuildRouteWeatherStatsJobFn: func(context.Context) (*model.Job, error) {
					return &model.Job{
						ID:     "job-wx-rebuild-1",
						Type:   model.JobTypeRebuildRouteWeatherStats,
						Status: model.JobStatusPending,
					}, nil
				},
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/rebuild/weather-stats", tt.body)
			rec := httptest.NewRecorder()
			h.Create(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
			}

			if checkedType != model.JobTypeRebuildRouteWeatherStats {
				t.Errorf("active job type = %q, want %q", checkedType, model.JobTypeRebuildRouteWeatherStats)
			}

			var resp RebuildWeatherStatsResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			if resp.Job.ID != "job-wx-rebuild-1" {
				t.Errorf("job id = %q, want job-wx-rebuild-1", resp.Job.ID)
			}

			if resp.Job.Type != model.JobTypeRebuildRouteWeatherStats {
				t.Errorf("job type = %q, want %q", resp.Job.Type, model.JobTypeRebuildRouteWeatherStats)
			}

			if resp.Job.Status != model.JobStatusPending {
				t.Errorf("status = %q, want pending", resp.Job.Status)
			}
		})
	}
}

func TestRebuildWeatherStatsActiveConflict(t *testing.T) {
	h := NewRebuildWeatherStatsHandler(&storetest.Stub{
		ActiveIngestJobFn: func(context.Context, model.JobType) (bool, error) {
			return true, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rebuild/weather-stats", nil)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	var body RebuildWeatherStatsConflictResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Error != errActiveRebuildJob {
		t.Errorf("error = %q, want %q", body.Error, errActiveRebuildJob)
	}

	if body.JobType != model.JobTypeRebuildRouteWeatherStats {
		t.Errorf("job_type = %q, want %q", body.JobType, model.JobTypeRebuildRouteWeatherStats)
	}
}

func TestRebuildWeatherStatsCreateConflict(t *testing.T) {
	h := NewRebuildWeatherStatsHandler(&storetest.Stub{
		ActiveIngestJobFn: func(context.Context, model.JobType) (bool, error) {
			return false, nil
		},
		CreateRebuildRouteWeatherStatsJobFn: func(context.Context) (*model.Job, error) {
			return nil, store.ErrActiveIngestConflict
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rebuild/weather-stats", nil)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	var body RebuildWeatherStatsConflictResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.JobType != model.JobTypeRebuildRouteWeatherStats {
		t.Errorf("job_type = %q, want %q", body.JobType, model.JobTypeRebuildRouteWeatherStats)
	}
}

func TestRebuildWeatherStatsRejectsParameters(t *testing.T) {
	h := NewRebuildWeatherStatsHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rebuild/weather-stats", bytes.NewBufferString(`{"force":true}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}
