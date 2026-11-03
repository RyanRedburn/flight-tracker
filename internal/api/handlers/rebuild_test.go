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

func TestRebuildCreate(t *testing.T) {
	endpoints := []struct {
		name    string
		path    string
		jobType model.JobType
		jobID   string
		create  func(store.Store) func(http.ResponseWriter, *http.Request)
		setJob  func(*storetest.Stub, func(context.Context) (*model.Job, error))
	}{
		{
			name:    "travel windows",
			path:    "/api/internal/rebuild/travel-windows",
			jobType: model.JobTypeRebuildRouteTravelWindows,
			jobID:   "job-rebuild-1",
			create: func(s store.Store) func(http.ResponseWriter, *http.Request) {
				return NewRebuildTravelWindowsHandler(s).Create
			},
			setJob: func(st *storetest.Stub, fn func(context.Context) (*model.Job, error)) {
				st.CreateRebuildRouteTravelWindowsJobFn = fn
			},
		},
		{
			name:    "weather stats",
			path:    "/api/internal/rebuild/weather-stats",
			jobType: model.JobTypeRebuildRouteWeatherStats,
			jobID:   "job-wx-rebuild-1",
			create: func(s store.Store) func(http.ResponseWriter, *http.Request) {
				return NewRebuildWeatherStatsHandler(s).Create
			},
			setJob: func(st *storetest.Stub, fn func(context.Context) (*model.Job, error)) {
				st.CreateRebuildRouteWeatherStatsJobFn = fn
			},
		},
	}

	tests := []struct {
		name     string
		raw      []byte
		empty    bool
		pending  bool
		conflict bool
		wantCode int
		wantJob  bool
	}{
		{name: "empty", empty: true, wantCode: http.StatusCreated, wantJob: true},
		{name: "empty object", raw: []byte("{}"), wantCode: http.StatusCreated, wantJob: true},
		{name: "active conflict", empty: true, pending: true, wantCode: http.StatusConflict},
		{name: "create conflict", empty: true, conflict: true, wantCode: http.StatusConflict},
		{name: "rejects parameters", raw: []byte(`{"force":true}`), wantCode: http.StatusBadRequest},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var checkedType model.JobType

					st := &storetest.Stub{}
					if tt.wantCode != http.StatusBadRequest {
						st.PendingIngestJobFn = func(_ context.Context, jobType model.JobType) (bool, error) {
							checkedType = jobType

							return tt.pending, nil
						}
					}

					if tt.wantJob || tt.conflict {
						ep.setJob(st, func(context.Context) (*model.Job, error) {
							if tt.conflict {
								return nil, store.ErrActiveIngestConflict
							}

							return &model.Job{
								ID:     ep.jobID,
								Type:   ep.jobType,
								Status: model.JobStatusPending,
							}, nil
						})
					}

					body := bytes.NewReader(tt.raw)
					if tt.empty {
						body = bytes.NewReader(nil)
					}

					req := httptest.NewRequest(http.MethodPost, ep.path, body)
					rec := httptest.NewRecorder()
					ep.create(st)(rec, req)

					if rec.Code != tt.wantCode {
						t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantCode, rec.Body.String())
					}

					if tt.wantCode != http.StatusBadRequest && checkedType != ep.jobType {
						t.Errorf("active job type = %q, want %q", checkedType, ep.jobType)
					}

					if tt.wantJob {
						var resp struct {
							Job QueuedJobResponse `json:"job"`
						}
						if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
							t.Fatalf("decode body: %v", err)
						}

						if resp.Job.ID != ep.jobID {
							t.Errorf("job id = %q, want %q", resp.Job.ID, ep.jobID)
						}

						if resp.Job.Type != ep.jobType {
							t.Errorf("job type = %q, want %q", resp.Job.Type, ep.jobType)
						}

						if resp.Job.Status != model.JobStatusPending {
							t.Errorf("status = %q, want pending", resp.Job.Status)
						}
					}

					if tt.wantCode == http.StatusConflict {
						var got struct {
							Error   string        `json:"error"`
							JobType model.JobType `json:"job_type"`
						}
						if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
							t.Fatalf("decode body: %v", err)
						}

						if got.Error != errActiveRebuildJob {
							t.Errorf("error = %q, want %q", got.Error, errActiveRebuildJob)
						}

						if got.JobType != ep.jobType {
							t.Errorf("job_type = %q, want %q", got.JobType, ep.jobType)
						}
					}
				})
			}
		})
	}
}
