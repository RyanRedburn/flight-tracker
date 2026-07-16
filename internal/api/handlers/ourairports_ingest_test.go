package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func ourAirportsSuccessStub() *storetest.Stub {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	return &storetest.Stub{
		ActiveIngestJobFn: func(context.Context, string) (bool, error) {
			return false, nil
		},
		HasOurAirportsDataFn: func(context.Context, store.OurAirportsDataset) (bool, error) {
			return false, nil
		},
		CreateOurAirportsIngestJobFn: func(_ context.Context, jt string) (*model.Job, error) {
			return &model.Job{
				ID:        "job-oa-1",
				Type:      jt,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
}

func postOurAirportsIngest(t *testing.T, h *OurAirportsIngestHandler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(http.MethodPost, path, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	switch path {
	case "/api/v1/ingest/countries":
		h.CreateCountries(rec, req)
	case "/api/v1/ingest/regions":
		h.CreateRegions(rec, req)
	case "/api/v1/ingest/airports":
		h.CreateAirports(rec, req)
	default:
		t.Fatalf("unexpected path %q", path)
	}

	return rec
}

func TestOurAirportsIngestCreateCountries(t *testing.T) {
	h := NewOurAirportsIngestHandler(ourAirportsSuccessStub())

	rec := postOurAirportsIngest(t, h, "/api/v1/ingest/countries", map[string]any{})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp ReferenceIngestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.Job.Type != model.JobTypeImportOurAirportsCountries {
		t.Errorf("job type = %q, want %q", resp.Job.Type, model.JobTypeImportOurAirportsCountries)
	}

	if resp.Job.Status != model.JobStatusPending {
		t.Errorf("status = %q, want pending", resp.Job.Status)
	}

	if resp.Job.ID == "" {
		t.Error("expected non-empty job id")
	}
}

func TestOurAirportsIngestCreateRegionsAndAirports(t *testing.T) {
	tests := []struct {
		path    string
		jobType string
	}{
		{"/api/v1/ingest/regions", model.JobTypeImportOurAirportsRegions},
		{"/api/v1/ingest/airports", model.JobTypeImportOurAirportsAirports},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			h := NewOurAirportsIngestHandler(ourAirportsSuccessStub())

			rec := postOurAirportsIngest(t, h, tt.path, map[string]any{})
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
			}

			var resp ReferenceIngestResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			if resp.Job.Type != tt.jobType {
				t.Errorf("job type = %q, want %q", resp.Job.Type, tt.jobType)
			}
		})
	}
}

func TestOurAirportsIngestActiveJobConflict(t *testing.T) {
	h := NewOurAirportsIngestHandler(&storetest.Stub{
		ActiveIngestJobFn: func(context.Context, string) (bool, error) {
			return true, nil
		},
	})

	rec := postOurAirportsIngest(t, h, "/api/v1/ingest/countries", map[string]any{})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["job_type"] != model.JobTypeImportOurAirportsCountries {
		t.Errorf("job_type = %v, want %q", body["job_type"], model.JobTypeImportOurAirportsCountries)
	}
}

func TestOurAirportsIngestExistingDataConflict(t *testing.T) {
	h := NewOurAirportsIngestHandler(&storetest.Stub{
		ActiveIngestJobFn: func(context.Context, string) (bool, error) {
			return false, nil
		},
		HasOurAirportsDataFn: func(context.Context, store.OurAirportsDataset) (bool, error) {
			return true, nil
		},
	})

	rec := postOurAirportsIngest(t, h, "/api/v1/ingest/countries", map[string]any{
		jsonForce: false,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["dataset"] != string(store.OurAirportsCountries) {
		t.Errorf("dataset = %v, want countries", body["dataset"])
	}
}

func TestOurAirportsIngestForceReimport(t *testing.T) {
	h := NewOurAirportsIngestHandler(ourAirportsSuccessStub())

	rec := postOurAirportsIngest(t, h, "/api/v1/ingest/countries", map[string]any{
		jsonForce: true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
}

func TestOurAirportsIngestInvalidJSON(t *testing.T) {
	h := NewOurAirportsIngestHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/countries", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	h.CreateCountries(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestOurAirportsIngestEmptyBody(t *testing.T) {
	h := NewOurAirportsIngestHandler(ourAirportsSuccessStub())

	rec := postOurAirportsIngest(t, h, "/api/v1/ingest/countries", nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 for empty body; body = %s", rec.Code, rec.Body.String())
	}
}

func TestOurAirportsIngestCompletedJobDoesNotBlock(t *testing.T) {
	// Scenario: ActiveIngestJob reports no active job (completed jobs do not block).
	h := NewOurAirportsIngestHandler(ourAirportsSuccessStub())

	rec := postOurAirportsIngest(t, h, "/api/v1/ingest/countries", map[string]any{})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 after completed job; body = %s", rec.Code, rec.Body.String())
	}
}
