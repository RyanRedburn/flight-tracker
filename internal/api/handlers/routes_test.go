package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const (
	testOriginORD     = "ORD"
	testDestLAX       = "LAX"
	testDestJFK       = "JFK"
	jsonCarrierOnTime = "carrier_on_time"
	testWindowStart   = "2024-04-30"
	testWindowEnd     = "2026-04-30"
	testAnalysisEnd   = "2026-04-15"
)

func TestRoutesStats(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Origin:    testOriginORD,
				Dest:      testDestLAX,
				Flights:   5,
				OnTime:    2,
				Delayed:   1,
				Cancelled: 1,
				Diverted:  1,
				DiversionAirports: []model.AirportCount{
					{Airport: "MDW", Count: 1},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ord&dest=lax&start_date=2026-04-01&end_date=2026-04-30&carrier=ua", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.Bytes()

	var stats model.RouteStats
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 5 {
		t.Fatalf("flights = %d, want 5", stats.Flights)
	}

	if stats.OnTime != 2 || stats.Delayed != 1 || stats.Cancelled != 1 || stats.Diverted != 1 {
		t.Fatalf("counts unexpected: %+v", stats)
	}

	if len(stats.DiversionAirports) != 1 || stats.DiversionAirports[0].Airport != "MDW" {
		t.Errorf("diversion_airports = %+v", stats.DiversionAirports)
	}

	if jsonHasKey(t, body, jsonCarrierOnTime) {
		t.Fatalf("carrier_on_time should be omitted when carrier is set: %s", body)
	}
}

func TestRoutesStatsEmpty(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return nil, store.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	assertNotFound(t, rec, "route stats not found")
}

func TestRoutesStatsEmptyFilters(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Origin:            testOriginORD,
				Dest:              testDestLAX,
				DiversionAirports: []model.AirportCount{},
				CarrierOnTime:     []model.CarrierOnTime{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	body := rec.Body.Bytes()

	var stats model.RouteStats
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 0 || stats.DiversionAirports == nil {
		t.Fatalf("empty stats = %+v", stats)
	}

	if stats.CarrierOnTime == nil {
		t.Fatal("carrier_on_time should be present when carrier is omitted")
	}

	if !jsonHasKey(t, body, jsonCarrierOnTime) {
		t.Fatalf("carrier_on_time missing from JSON: %s", body)
	}
}

func TestRoutesStatsStoreError(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return nil, context.DeadlineExceeded
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestRoutesStatsCarrierOnTime(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(_ context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
			if filter.Carrier != "" {
				t.Fatalf("carrier = %q, want empty", filter.Carrier)
			}

			return &model.RouteStats{
				Origin:  testOriginORD,
				Dest:    testDestLAX,
				Flights: 10,
				OnTime:  8,
				Delayed: 2,
				CarrierOnTime: []model.CarrierOnTime{
					{Carrier: "UA", OnTimeRate: 0.83, Flights: 6},
					{Carrier: "AA", OnTimeRate: 0.75, Flights: 4},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var stats model.RouteStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(stats.CarrierOnTime) != 2 {
		t.Fatalf("carrier_on_time len = %d, want 2", len(stats.CarrierOnTime))
	}

	if stats.CarrierOnTime[0].Carrier != "UA" || stats.CarrierOnTime[0].OnTimeRate != 0.83 {
		t.Errorf("first row = %+v", stats.CarrierOnTime[0])
	}

	if stats.CarrierOnTime[1].Carrier != "AA" || stats.CarrierOnTime[1].Flights != 4 {
		t.Errorf("second row = %+v", stats.CarrierOnTime[1])
	}
}

func TestRoutesStatsBadRequest(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRoutesOutlook(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return &model.RouteOutlook{
				Origin:             testOriginORD,
				Dest:               testDestLAX,
				Carrier:            "UA",
				DayOfWeek:          3,
				DepTime:            "0700",
				SampleSize:         3,
				InsufficientSample: true,
				AnalysisEnd:        testAnalysisEnd,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3&dep_time=0700", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var out model.RouteOutlook
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.SampleSize != 3 {
		t.Fatalf("sample_size = %d, want 3", out.SampleSize)
	}

	if !out.InsufficientSample {
		t.Fatal("expected insufficient_sample")
	}

	if out.AnalysisEnd != testAnalysisEnd {
		t.Errorf("analysis_end = %q, want %q", out.AnalysisEnd, testAnalysisEnd)
	}
}

func TestRoutesOutlookEmpty(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return nil, store.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3&dep_time=0700", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	assertNotFound(t, rec, "route outlook not found")
}

func TestRoutesOutlookEmptyFilters(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return &model.RouteOutlook{
				Origin:        testOriginORD,
				Dest:          testDestLAX,
				Carrier:       "UA",
				DayOfWeek:     3,
				DepTime:       "0700",
				SampleSize:    0,
				AnalysisStart: "2025-04-15",
				AnalysisEnd:   testAnalysisEnd,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3&dep_time=0700", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var out model.RouteOutlook
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.SampleSize != 0 {
		t.Fatalf("sample_size = %d, want 0", out.SampleSize)
	}

	if out.AnalysisEnd != testAnalysisEnd {
		t.Errorf("analysis_end = %q, want %q", out.AnalysisEnd, testAnalysisEnd)
	}
}

func TestRoutesOutlookStoreError(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return nil, context.DeadlineExceeded
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3&dep_time=0700", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestRoutesTravelWindows(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteTravelWindowsFn: func(_ context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
			if filter.Origin != testOriginORD || filter.Dest != testDestLAX {
				t.Fatalf("filter = %+v", filter)
			}

			if filter.Carrier != "" {
				t.Fatalf("carrier = %q, want empty", filter.Carrier)
			}

			return &model.RouteTravelWindows{
				Origin:      testOriginORD,
				Dest:        testDestLAX,
				WindowStart: testWindowStart,
				WindowEnd:   testWindowEnd,
				ByMonth: []model.TravelWindowMonthBucket{
					{Month: 1, OnTimeRate: 0.8, Flights: 100},
				},
				ByDayOfWeek: []model.TravelWindowDayBucket{},
				ByHour:      []model.TravelWindowHourBucket{},
				BestMonths:  []model.TravelWindowMonthBucket{{Month: 1, OnTimeRate: 0.8, Flights: 100}},
				WorstMonths: []model.TravelWindowMonthBucket{{Month: 1, OnTimeRate: 0.8, Flights: 100}},
				BestDays:    []model.TravelWindowDayBucket{},
				WorstDays:   []model.TravelWindowDayBucket{},
				BestHours:   []model.TravelWindowHourBucket{},
				WorstHours:  []model.TravelWindowHourBucket{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/travel-windows?origin=ord&dest=lax", nil)
	rec := httptest.NewRecorder()
	h.TravelWindows(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.Bytes()

	var out model.RouteTravelWindows
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.Origin != testOriginORD || out.Dest != testDestLAX {
		t.Fatalf("od = %s-%s", out.Origin, out.Dest)
	}

	if out.WindowStart != testWindowStart || out.WindowEnd != testWindowEnd {
		t.Errorf("window = %s..%s", out.WindowStart, out.WindowEnd)
	}

	if jsonHasKey(t, body, "carrier") {
		t.Fatalf("carrier should be omitted when not requested: %s", body)
	}

	if len(out.ByMonth) != 1 || out.ByMonth[0].Month != 1 {
		t.Errorf("by_month = %+v", out.ByMonth)
	}
}

func TestRoutesTravelWindowsCarrier(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteTravelWindowsFn: func(_ context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
			if filter.Carrier != "UA" {
				t.Fatalf("carrier = %q, want UA", filter.Carrier)
			}

			return &model.RouteTravelWindows{
				Origin:      testOriginORD,
				Dest:        testDestLAX,
				Carrier:     "UA",
				WindowStart: "2025-01-01",
				WindowEnd:   testWindowEnd,
				ByMonth:     []model.TravelWindowMonthBucket{},
				ByDayOfWeek: []model.TravelWindowDayBucket{},
				ByHour:      []model.TravelWindowHourBucket{},
				BestMonths:  []model.TravelWindowMonthBucket{},
				WorstMonths: []model.TravelWindowMonthBucket{},
				BestDays:    []model.TravelWindowDayBucket{},
				WorstDays:   []model.TravelWindowDayBucket{},
				BestHours:   []model.TravelWindowHourBucket{},
				WorstHours:  []model.TravelWindowHourBucket{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/travel-windows?origin=ORD&dest=LAX&carrier=ua", nil)
	rec := httptest.NewRecorder()
	h.TravelWindows(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.Bytes()

	var out model.RouteTravelWindows
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.Carrier != "UA" {
		t.Fatalf("carrier = %q, want UA", out.Carrier)
	}

	if !jsonHasKey(t, body, "carrier") {
		t.Fatalf("carrier missing from JSON: %s", body)
	}
}

func TestRoutesTravelWindowsNotFound(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteTravelWindowsFn: func(context.Context, store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
			return nil, store.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/travel-windows?origin=ORD&dest=LAX", nil)
	rec := httptest.NewRecorder()
	h.TravelWindows(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRoutesTravelWindowsBadRequest(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/travel-windows?origin=ORD", nil)
	rec := httptest.NewRecorder()
	h.TravelWindows(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRoutesOutlookBadRequest(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func assertNotFound(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Error != want {
		t.Fatalf("error = %q, want %q", resp.Error, want)
	}
}

func jsonHasKey(t *testing.T, body []byte, field string) bool {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	_, ok := raw[field]

	return ok
}
