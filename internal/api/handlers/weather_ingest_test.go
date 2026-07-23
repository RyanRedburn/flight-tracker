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

const jsonStations = "stations"

func weatherIngestSuccessStub() *storetest.Stub {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	return &storetest.Stub{
		ActiveWeatherIngestMonthsFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return nil, nil
		},
		MonthsWithWeatherDataFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return nil, nil
		},
		CreateWeatherIngestJobFn: func(_ context.Context, year, month int, stations []string) (*model.Job, error) {
			return &model.Job{
				ID:        "job-weather",
				Type:      model.JobTypeImportWeatherObservations,
				Status:    model.JobStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
}

func postWeatherIngest(t *testing.T, h *WeatherIngestHandler, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/weather", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	return rec
}

func TestWeatherIngestSingleMonth(t *testing.T) {
	st := weatherIngestSuccessStub()

	var createdYear, createdMonth int
	var createdStations []string

	st.CreateWeatherIngestJobFn = func(_ context.Context, year, month int, stations []string) (*model.Job, error) {
		createdYear, createdMonth = year, month
		createdStations = append([]string(nil), stations...)
		now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		return &model.Job{
			ID:        "job-1",
			Type:      model.JobTypeImportWeatherObservations,
			Status:    model.JobStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	}

	h := NewWeatherIngestHandler(st, defaultMaxIngestMonths)

	rec := postWeatherIngest(t, h, map[string]any{
		jsonStartYear:  2024,
		jsonStartMonth: 1,
		jsonStations:   []string{"ord", "JFK"},
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp WeatherIngestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.MonthsRequested != 1 || len(resp.Jobs) != 1 {
		t.Fatalf("response = %+v, want 1 job", resp)
	}

	if createdYear != 2024 || createdMonth != 1 {
		t.Errorf("CreateWeatherIngestJob args = %d-%d, want 2024-1", createdYear, createdMonth)
	}

	if len(createdStations) != 2 || createdStations[0] != "ORD" || createdStations[1] != "JFK" {
		t.Errorf("stations = %v, want [ORD JFK]", createdStations)
	}

	if resp.Jobs[0].Stations[0] != "ORD" || resp.Jobs[0].Status != model.JobStatusPending {
		t.Errorf("job = %+v", resp.Jobs[0])
	}
}

func TestWeatherIngestRange(t *testing.T) {
	st := weatherIngestSuccessStub()

	var createCalls int

	st.CreateWeatherIngestJobFn = func(_ context.Context, year, month int, stations []string) (*model.Job, error) {
		createCalls++
		now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		return &model.Job{
			ID:        fmt.Sprintf("job-%d", createCalls),
			Type:      model.JobTypeImportWeatherObservations,
			Status:    model.JobStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	}

	h := NewWeatherIngestHandler(st, defaultMaxIngestMonths)

	rec := postWeatherIngest(t, h, map[string]any{
		jsonStartYear:  2024,
		jsonStartMonth: 1,
		jsonEndYear:    2024,
		jsonEndMonth:   3,
		jsonStations:   []string{"ORD"},
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp WeatherIngestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.MonthsRequested != 3 || len(resp.Jobs) != 3 || createCalls != 3 {
		t.Fatalf("response = %+v, createCalls = %d, want 3 jobs", resp, createCalls)
	}
}

func TestWeatherIngestMissingStations(t *testing.T) {
	h := NewWeatherIngestHandler(&storetest.Stub{}, defaultMaxIngestMonths)

	rec := postWeatherIngest(t, h, map[string]any{
		jsonStartYear:  2024,
		jsonStartMonth: 1,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestWeatherIngestActiveJobConflict(t *testing.T) {
	h := NewWeatherIngestHandler(&storetest.Stub{
		ActiveWeatherIngestMonthsFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return []model.YearMonth{{Year: 2024, Month: 1}}, nil
		},
	}, defaultMaxIngestMonths)

	rec := postWeatherIngest(t, h, map[string]any{
		jsonStartYear:  2024,
		jsonStartMonth: 1,
		jsonStations:   []string{"ORD"},
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
}

func TestWeatherIngestExistingDataConflict(t *testing.T) {
	h := NewWeatherIngestHandler(&storetest.Stub{
		ActiveWeatherIngestMonthsFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return nil, nil
		},
		MonthsWithWeatherDataFn: func(context.Context, []model.YearMonth) ([]model.YearMonth, error) {
			return []model.YearMonth{{Year: 2024, Month: 1}}, nil
		},
	}, defaultMaxIngestMonths)

	rec := postWeatherIngest(t, h, map[string]any{
		jsonStartYear:  2024,
		jsonStartMonth: 1,
		jsonStations:   []string{"ORD"},
		jsonForce:      false,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
}

func TestWeatherIngestForceReimport(t *testing.T) {
	h := NewWeatherIngestHandler(weatherIngestSuccessStub(), defaultMaxIngestMonths)

	rec := postWeatherIngest(t, h, map[string]any{
		jsonStartYear:  2024,
		jsonStartMonth: 1,
		jsonStations:   []string{"ORD"},
		jsonForce:      true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
}
