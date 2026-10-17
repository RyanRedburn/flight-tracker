package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestRouteReliabilityPrefersHigherOnTime(t *testing.T) {
	stub := &storetest.Stub{
		SearchAirportsFn: exactAirportSearch,
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			rate := 0.6
			if filter.DepTime == "0700" {
				rate = 0.85
			}

			return &model.RouteOutlook{
				Filters: model.Filters{DepTime: filter.DepTime},
				OutlookResult: model.OutlookResult{
					Analysis: model.Analysis{
						Start:        testStatsStart,
						End:          testStatsEnd,
						SampleSize:   30,
						SampleReason: model.SampleReasonSufficient,
						Confidence:   model.ConfidenceHigh,
					},
					OnTimeProbability: &rate,
				},
			}, nil
		},
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Flights:  120,
				Analysis: model.Analysis{SampleSize: 120, SampleReason: model.SampleReasonSufficient, Confidence: model.ConfidenceHigh},
			}, nil
		},
	}

	res := callTool(t, stub, toolRouteReliability, map[string]any{
		jsonKeyOrigin:  testCodeORD,
		jsonKeyDest:    testCodeLAX,
		jsonKeyCarrier: testCarrierUA,
		jsonKeyDate:    testDateOct6,
		"dep_times":    []string{"0700", "1400"},
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[routeReliabilityData](t, res)
	if !containsAll(out.Summary, "0700", "1400", "Prefer") {
		t.Fatalf("summary = %q", out.Summary)
	}

	if len(out.Data.Departures) != 2 {
		t.Fatalf("departures = %d", len(out.Data.Departures))
	}
}

func TestRouteReliabilityNoDataInWindow(t *testing.T) {
	stub := &storetest.Stub{
		SearchAirportsFn: exactAirportSearch,
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			rate := 0.5

			return &model.RouteOutlook{
				OutlookResult: model.OutlookResult{
					Analysis:          model.Analysis{Start: testStatsStart, End: testStatsEnd, SampleSize: 10, Confidence: model.ConfidenceLow},
					OnTimeProbability: &rate,
				},
			}, nil
		},
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{Flights: 0, Analysis: model.Analysis{Start: "2025-01-01", End: "2026-01-01"}}, nil
		},
		RouteFlightDateRangeFn: func(context.Context, string, string, string) (store.RouteFlightDateRange, error) {
			return store.RouteFlightDateRange{Start: "2024-06-01", End: "2026-05-31"}, nil
		},
	}

	res := callTool(t, stub, toolRouteReliability, map[string]any{
		jsonKeyOrigin:  testCodeORD,
		jsonKeyDest:    testCodeLAX,
		jsonKeyCarrier: testCarrierUA,
		"day_of_week":  2,
		"dep_times":    []string{"0700"},
	})
	if !res.IsError {
		t.Fatal("expected error")
	}

	out := decodeResult[routeReliabilityData](t, res)
	if out.Error == nil || out.Error.Code != CodeNoDataInWindow {
		t.Fatalf("error = %#v", out.Error)
	}

	if out.Error.AvailableStart != "2024-06-01" || out.Error.AvailableEnd != "2026-05-31" {
		t.Fatalf("available = %q..%q", out.Error.AvailableStart, out.Error.AvailableEnd)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}

	return true
}
