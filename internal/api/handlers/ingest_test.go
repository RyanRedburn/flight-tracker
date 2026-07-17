package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const (
	defaultMaxIngestMonths = 24
	jsonStartYear          = "start_year"
	jsonStartMonth         = "start_month"
	jsonEndYear            = "end_year"
	jsonEndMonth           = "end_month"
	jsonForce              = "force"
)

func ingestSuccessStub() *storetest.Stub {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	return &storetest.Stub{
		ActiveFlightPerformanceIngestMonthsFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return nil, nil
		},
		MonthsWithFlightPerformanceDataFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return nil, nil
		},
		CreateFlightPerformanceIngestJobFn: func(_ context.Context, year, month int) (*model.Job, error) {
			return &model.Job{
				ID:        "job-bts",
				Type:      model.JobTypeImportFlightPerformance,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
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
	st := ingestSuccessStub()

	var createdYear, createdMonth int

	st.CreateFlightPerformanceIngestJobFn = func(_ context.Context, year, month int) (*model.Job, error) {
		createdYear, createdMonth = year, month
		now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

		return &model.Job{
			ID:        "job-1",
			Type:      model.JobTypeImportFlightPerformance,
			Status:    model.JobStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	}

	h := NewIngestHandler(st, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 4,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp IngestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.MonthsRequested != 1 {
		t.Errorf("MonthsRequested = %d, want 1", resp.MonthsRequested)
	}

	if len(resp.Jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(resp.Jobs))
	}

	if createdYear != 2026 || createdMonth != 4 {
		t.Errorf("CreateFlightPerformanceIngestJob args = %d-%d, want 2026-4", createdYear, createdMonth)
	}

	if resp.Jobs[0].Year != 2026 || resp.Jobs[0].Month != 4 {
		t.Errorf("job = %+v, want 2026-04", resp.Jobs[0])
	}

	if resp.Jobs[0].Status != model.JobStatusPending {
		t.Errorf("status = %q, want pending", resp.Jobs[0].Status)
	}
}

func TestIngestRange(t *testing.T) {
	st := ingestSuccessStub()

	var createCalls int

	st.CreateFlightPerformanceIngestJobFn = func(_ context.Context, year, month int) (*model.Job, error) {
		createCalls++
		now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

		return &model.Job{
			ID:        fmt.Sprintf("job-%d", createCalls),
			Type:      model.JobTypeImportFlightPerformance,
			Status:    model.JobStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	}

	h := NewIngestHandler(st, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 1,
		jsonEndYear:    2026,
		jsonEndMonth:   3,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp IngestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.MonthsRequested != 3 || len(resp.Jobs) != 3 || createCalls != 3 {
		t.Fatalf("response = %+v, createCalls = %d, want 3 jobs", resp, createCalls)
	}
}

func TestIngestInvalidMonth(t *testing.T) {
	h := NewIngestHandler(&storetest.Stub{}, defaultMaxIngestMonths)

	rec := postIngest(t, h, map[string]any{
		jsonStartYear:  2026,
		jsonStartMonth: 13,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIngestRangeTooLarge(t *testing.T) {
	h := NewIngestHandler(&storetest.Stub{}, 2)

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
	h := NewIngestHandler(&storetest.Stub{
		ActiveFlightPerformanceIngestMonthsFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return []model.YearMonth{{Year: 2026, Month: 4}}, nil
		},
	}, defaultMaxIngestMonths)

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
	h := NewIngestHandler(&storetest.Stub{
		ActiveFlightPerformanceIngestMonthsFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return nil, nil
		},
		MonthsWithFlightPerformanceDataFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return []model.YearMonth{{Year: 2026, Month: 4}}, nil
		},
	}, defaultMaxIngestMonths)

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
	h := NewIngestHandler(ingestSuccessStub(), defaultMaxIngestMonths)

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
	h := NewIngestHandler(&storetest.Stub{}, defaultMaxIngestMonths)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIngestCompletedJobDoesNotBlock(t *testing.T) {
	// Scenario: no active ingest months (completed jobs are not "active").
	h := NewIngestHandler(ingestSuccessStub(), defaultMaxIngestMonths)

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
	h := NewIngestHandler(&storetest.Stub{}, defaultMaxIngestMonths)

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
