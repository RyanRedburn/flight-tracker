package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"

	"github.com/go-chi/chi/v5"
)

const (
	defaultMaxIngestMonths = 24
	jsonStartYear          = "start_year"
	jsonStartMonth         = "start_month"
	jsonEndYear            = "end_year"
	jsonEndMonth           = "end_month"
	jsonForce              = "force"
)

func newTestIngestHandler(t *testing.T, maxMonths int) (*IngestHandler, *mem.Store) {
	t.Helper()

	if maxMonths < 1 {
		maxMonths = defaultMaxIngestMonths
	}

	s := mem.New()

	return NewIngestHandler(s, maxMonths), s
}

func postIngest(t *testing.T, h *IngestHandler, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	return rec
}

func TestIngestSingleMonth(t *testing.T) {
	h, _ := newTestIngestHandler(t, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 4,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp ingestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.MonthsRequested != 1 {
		t.Errorf("MonthsRequested = %d, want 1", resp.MonthsRequested)
	}

	if len(resp.Jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(resp.Jobs))
	}

	if resp.Jobs[0].Year != 2026 || resp.Jobs[0].Month != 4 {
		t.Errorf("job = %+v, want 2026-04", resp.Jobs[0])
	}

	if resp.Jobs[0].Status != model.JobStatusPending {
		t.Errorf("status = %q, want pending", resp.Jobs[0].Status)
	}
}

func TestIngestRange(t *testing.T) {
	h, _ := newTestIngestHandler(t, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 1,
		jsonEndYear:    2026,
		jsonEndMonth:   3,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp ingestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.MonthsRequested != 3 || len(resp.Jobs) != 3 {
		t.Fatalf("response = %+v, want 3 jobs", resp)
	}
}

func TestIngestInvalidMonth(t *testing.T) {
	h, _ := newTestIngestHandler(t, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 13,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIngestRangeTooLarge(t *testing.T) {
	h, _ := newTestIngestHandler(t, 2)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 1,
		jsonEndYear:    2026,
		jsonEndMonth:   4,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIngestActiveJobConflict(t *testing.T) {
	h, s := newTestIngestHandler(t, defaultMaxIngestMonths)
	ctx := context.Background()

	if _, err := s.CreateBTSIngestJob(ctx, 2026, 4); err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 4,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["active_ingest_months"] == nil {
		t.Fatal("expected active_ingest_months in response")
	}
}

func TestIngestExistingDataConflict(t *testing.T) {
	h, s := newTestIngestHandler(t, defaultMaxIngestMonths)
	ctx := context.Background()

	columns := []string{"flight_date", "origin", "dest"}

	rows := [][]string{{testFlightDate20260424, testAirportORD, testAirportBHM}}
	if err := s.ReplaceOnTimeFlightsByMonth(ctx, 2026, 4, columns, rows); err != nil {
		t.Fatalf("ReplaceOnTimeFlightsByMonth() error = %v", err)
	}

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 4,
		jsonForce:      false,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["existing_data_months"] == nil {
		t.Fatal("expected existing_data_months in response")
	}
}

func TestIngestForceReimport(t *testing.T) {
	h, s := newTestIngestHandler(t, defaultMaxIngestMonths)
	ctx := context.Background()

	columns := []string{"flight_date", "origin", "dest"}

	rows := [][]string{{testFlightDate20260424, testAirportORD, testAirportBHM}}
	if err := s.ReplaceOnTimeFlightsByMonth(ctx, 2026, 4, columns, rows); err != nil {
		t.Fatalf("ReplaceOnTimeFlightsByMonth() error = %v", err)
	}

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 4,
		jsonForce:      true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
}

func TestIngestInvalidJSON(t *testing.T) {
	h, _ := newTestIngestHandler(t, defaultMaxIngestMonths)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIngestCompletedJobDoesNotBlock(t *testing.T) {
	h, s := newTestIngestHandler(t, defaultMaxIngestMonths)
	ctx := context.Background()

	job, err := s.CreateBTSIngestJob(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	claimed, err := s.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if err := s.CompleteJob(ctx, claimed.ID, []byte(`{"rows_imported":1}`)); err != nil {
		t.Fatalf("CompleteJob() error = %v", err)
	}

	_ = job

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 4,
		jsonForce:      true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 after completed job; body = %s", rec.Code, rec.Body.String())
	}
}

func TestIngestEndBeforeStart(t *testing.T) {
	h, _ := newTestIngestHandler(t, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 6,
		jsonEndYear:    2026,
		jsonEndMonth:   1,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIngestStoresQueryableJobs(t *testing.T) {
	h, s := newTestIngestHandler(t, defaultMaxIngestMonths)
	jobsHandler := NewJobsHandler(s)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 5,
	})

	var created ingestResponse
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+created.Jobs[0].ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", created.Jobs[0].ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	listRec := httptest.NewRecorder()
	jobsHandler.Get(listRec, req)

	if listRec.Code != http.StatusOK {
		t.Fatalf("Get job status = %d, want 200", listRec.Code)
	}
}
