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
		jsonKeyOrigin:   testCodeORD,
		jsonKeyDest:     testCodeLAX,
		jsonKeyCarrier:  testCarrierUA,
		jsonKeyDate:     testDateOct6,
		jsonKeyDepTimes: []string{"0700", "1400"},
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
		jsonKeyOrigin:    testCodeORD,
		jsonKeyDest:      testCodeLAX,
		jsonKeyCarrier:   testCarrierUA,
		jsonKeyDayOfWeek: 2,
		jsonKeyDepTimes:  []string{"0700"},
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

func TestRouteReliabilityNarrowsFlightNumberByDepTime(t *testing.T) {
	var got store.RouteOutlookFilter

	stub := &storetest.Stub{
		SearchAirportsFn: exactAirportSearch,
		FlightNumberRoutesFn: func(_ context.Context, filter store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			if filter.Carrier != testCarrierUA || filter.FlightNumber != "200" {
				t.Errorf("flight filter = %+v", filter)
			}

			return []model.FlightNumberRoute{
				{Origin: testCodeORD, Dest: testCodeLAX, Flights: 40, UsualDepTime: "0700"},
				{Origin: testCodeORD, Dest: testCodeSFO, Flights: 30, UsualDepTime: "1800"},
			}, nil
		},
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			got = filter
			rate := 0.8

			return &model.RouteOutlook{
				OutlookResult: model.OutlookResult{
					Analysis:          model.Analysis{Start: testStatsStart, End: testStatsEnd, SampleSize: 20, Confidence: model.ConfidenceHigh},
					OnTimeProbability: &rate,
				},
			}, nil
		},
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Flights:  40,
				Analysis: model.Analysis{SampleSize: 40, SampleReason: model.SampleReasonSufficient, Confidence: model.ConfidenceHigh},
			}, nil
		},
	}

	res := callTool(t, stub, toolRouteReliability, map[string]any{
		jsonKeyCarrier:      testCarrierUA,
		jsonKeyFlightNumber: "200",
		jsonKeyDayOfWeek:    2,
		jsonKeyDepTimes:     []string{"1810"},
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	if got.Origin != testCodeORD || got.Dest != testCodeSFO || got.DepTime != "1810" {
		t.Fatalf("filter = %+v", got)
	}
}

func TestRouteReliabilityRouteNotFound(t *testing.T) {
	tests := []struct {
		name string
		stub *storetest.Stub
	}{
		{
			name: "outlook missing",
			stub: &storetest.Stub{
				SearchAirportsFn: exactAirportSearch,
				RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
					return nil, store.ErrNotFound
				},
			},
		},
		{
			name: "no flights at all",
			stub: &storetest.Stub{
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
					return &model.RouteStats{Flights: 0}, nil
				},
				RouteFlightDateRangeFn: func(context.Context, string, string, string) (store.RouteFlightDateRange, error) {
					return store.RouteFlightDateRange{}, store.ErrNotFound
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := callTool(t, tt.stub, toolRouteReliability, map[string]any{
				jsonKeyOrigin:   testCodeORD,
				jsonKeyDest:     testCodeLAX,
				jsonKeyCarrier:  testCarrierUA,
				jsonKeyDate:     testDateOct6,
				jsonKeyDepTimes: []string{"0700"},
			})
			if !res.IsError {
				t.Fatal("expected error")
			}

			out := decodeResult[routeReliabilityData](t, res)
			if out.Error == nil || out.Error.Code != CodeRouteNotFound {
				t.Fatalf("error = %#v", out.Error)
			}
		})
	}
}

func TestRouteReliabilityBadInput(t *testing.T) {
	base := routeReliabilityInput{
		Origin:   testCodeORD,
		Dest:     testCodeLAX,
		Carrier:  testCarrierUA,
		Date:     testDateOct6,
		DepTimes: []string{"0700"},
	}
	tests := []struct {
		name string
		edit func(*routeReliabilityInput)
	}{
		{name: "only origin", edit: func(in *routeReliabilityInput) { in.Dest = "" }},
		{name: "only dest", edit: func(in *routeReliabilityInput) { in.Origin = "" }},
		{name: "both date and day", edit: func(in *routeReliabilityInput) { in.DayOfWeek = 2 }},
		{name: "neither date nor day", edit: func(in *routeReliabilityInput) { in.Date = "" }},
		{name: "bad dep time", edit: func(in *routeReliabilityInput) { in.DepTimes = []string{"2460"} }},
		{name: "empty dep times", edit: func(in *routeReliabilityInput) { in.DepTimes = nil }},
	}

	stub := &storetest.Stub{SearchAirportsFn: exactAirportSearch}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.edit(&in)

			res, out, err := routeReliability(context.Background(), stub, testLogger(), in)
			if err != nil {
				t.Fatalf("routeReliability: %v", err)
			}

			if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeInvalidInput {
				t.Fatalf("error = %#v", out.Error)
			}
		})
	}
}

func TestRouteReliabilityInputSchema(t *testing.T) {
	schema := routeReliabilityInputSchema()
	assertCodeSchema(t, schema.Properties["carrier"], 2)
	assertCodeSchema(t, schema.Properties["origin"], 3)
	assertCodeSchema(t, schema.Properties["dest"], 3)

	flightNumber := schema.Properties[jsonKeyFlightNumber]
	if flightNumber == nil || flightNumber.MaxLength == nil || *flightNumber.MaxLength != store.FlightNumberMaxLen {
		t.Fatalf("flight_number schema = %#v", flightNumber)
	}

	day := schema.Properties[jsonKeyDayOfWeek]
	if day == nil || day.Minimum == nil || *day.Minimum != 1 || day.Maximum == nil || *day.Maximum != 7 {
		t.Fatalf("day_of_week schema = %#v", day)
	}

	depTimes := schema.Properties[jsonKeyDepTimes]
	if depTimes == nil || depTimes.MinItems == nil || *depTimes.MinItems != routeReliabilityMinDepTimes || depTimes.MaxItems == nil || *depTimes.MaxItems != routeReliabilityMaxDepTimes {
		t.Fatalf("dep_times schema = %#v", depTimes)
	}

	window := schema.Properties["dep_time_window_minutes"]
	if window == nil || window.Minimum == nil || *window.Minimum != 1 || window.Maximum == nil || *window.Maximum != float64(store.MaxDepTimeWindowMinutes) {
		t.Fatalf("window schema = %#v", window)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	valid := map[string]any{
		jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
		jsonKeyDate: testDateOct6, jsonKeyDepTimes: []string{"0700", "1400"},
	}
	if err := resolved.Validate(valid); err != nil {
		t.Fatalf("valid args: %v", err)
	}

	tooMany := map[string]any{
		jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
		jsonKeyDate: testDateOct6, jsonKeyDepTimes: []string{"0700", "0800", "0900", "1000", "1100", "1200"},
	}
	if err := resolved.Validate(tooMany); err == nil {
		t.Fatal("expected validation error for too many dep_times")
	}

	badDay := map[string]any{
		jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
		jsonKeyDayOfWeek: 8, jsonKeyDepTimes: []string{"0700"},
	}
	if err := resolved.Validate(badDay); err == nil {
		t.Fatal("expected validation error for day_of_week")
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
