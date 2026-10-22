package mcp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const latestCarrierDate = "2026-04-01"

func TestCompareCarriersSummaries(t *testing.T) {
	minFlights := store.CarrierStatsMinSampleSize

	windowStart, windowEnd, ok := store.TrailingOutlookWindow(latestCarrierDate)
	if !ok || windowEnd != latestCarrierDate {
		t.Fatalf("window = %s %s ok=%v", windowStart, windowEnd, ok)
	}

	tests := []struct {
		name      string
		args      map[string]any
		carriers  []model.CarrierOnTime
		want      string
		wantConf  string
		wantFirst string
		wantStart string
		wantEnd   string
	}{
		{
			name: "ten point gap on uneven samples is too close",
			carriers: []model.CarrierOnTime{
				{Carrier: "NK", OnTimeRate: 0.77, Flights: 120},
				{Carrier: "F9", OnTimeRate: 0.67, Flights: 180},
			},
			want:      "NK and F9 are too close to call on ORD–LAX (77% and 67% on time, medium confidence, 120 and 180 flights).",
			wantConf:  model.ConfidenceMedium,
			wantFirst: "NK",
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "later carrier beats the first",
			carriers: []model.CarrierOnTime{
				{Carrier: "AA", OnTimeRate: 0.60, Flights: 300},
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: 320},
			},
			want:      "UA is more reliable than AA on ORD–LAX (85% vs 60% on time, high confidence, 320 and 300 flights).",
			wantConf:  model.ConfidenceHigh,
			wantFirst: "AA",
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "exact tie names both carriers",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.80, Flights: 300},
				{Carrier: "AA", OnTimeRate: 0.80, Flights: 310},
			},
			want:      "UA and AA are too close to call on ORD–LAX (80% on time, high confidence, 300 and 310 flights).",
			wantConf:  model.ConfidenceHigh,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "near tie names both carriers",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.82, Flights: 310},
				{Carrier: "AA", OnTimeRate: 0.80, Flights: 300},
			},
			want:      "UA and AA are too close to call on ORD–LAX (82% and 80% on time, high confidence, 310 and 300 flights).",
			wantConf:  model.ConfidenceHigh,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "steady slide names every carrier",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: 320},
				{Carrier: "AA", OnTimeRate: 0.83, Flights: 310},
				{Carrier: "DL", OnTimeRate: 0.81, Flights: 300},
			},
			want:      "UA, AA and DL are too close to call on ORD–LAX (85%, 83% and 81% on time, high confidence, 320, 310 and 300 flights).",
			wantConf:  model.ConfidenceHigh,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "two lead and one trails",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: 320},
				{Carrier: "AA", OnTimeRate: 0.84, Flights: 310},
				{Carrier: "DL", OnTimeRate: 0.70, Flights: 300},
			},
			want:      "UA and AA are too close to call and more reliable than DL on ORD–LAX (85% and 84% vs 70% on time, high confidence, 320, 310 and 300 flights).",
			wantConf:  model.ConfidenceHigh,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "one solid carrier and every thin carrier",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: minFlights},
				{Carrier: "AA", OnTimeRate: 0.95, Flights: 8},
				{Carrier: "DL", OnTimeRate: 0.40, Flights: 4},
			},
			want:      fmt.Sprintf("UA is the only carrier with enough data on ORD–LAX (85%% on time, medium confidence, %d flights). AA (low confidence, 8 flights) and DL (low confidence, 4 flights) have thin data.", minFlights),
			wantConf:  model.ConfidenceMedium,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "every thin carrier is named",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.90, Flights: minFlights - 1},
				{Carrier: "AA", OnTimeRate: 0.50, Flights: 8},
				{Carrier: "DL", OnTimeRate: 0.20, Flights: 4},
			},
			want:      fmt.Sprintf("No carrier on ORD–LAX has enough data to compare. UA (low confidence, %d flights), AA (low confidence, 8 flights) and DL (low confidence, 4 flights) have thin data.", minFlights-1),
			wantConf:  model.ConfidenceLow,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "only carrier on the route",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: minFlights},
			},
			want:      fmt.Sprintf("UA is on time about 85%% of the time on ORD–LAX (medium confidence, %d flights).", minFlights),
			wantConf:  model.ConfidenceMedium,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "one flight stays singular",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 1, Flights: 1},
			},
			want:      "No carrier on ORD–LAX has enough data to compare. UA (low confidence, 1 flight) has thin data.",
			wantConf:  model.ConfidenceLow,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "blank carrier code is still named",
			carriers: []model.CarrierOnTime{
				{Carrier: "", OnTimeRate: 0.80, Flights: minFlights},
				{Carrier: "AA", OnTimeRate: 0.70, Flights: 4},
			},
			want:      fmt.Sprintf("An unknown carrier is the only carrier with enough data on ORD–LAX (80%% on time, medium confidence, %d flights). AA (low confidence, 4 flights) has thin data.", minFlights),
			wantConf:  model.ConfidenceMedium,
			wantFirst: "",
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "unknown carrier is lowercase mid-sentence",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: 320},
				{Carrier: "", OnTimeRate: 0.60, Flights: 300},
			},
			want:      "UA is more reliable than an unknown carrier on ORD–LAX (85% vs 60% on time, high confidence, 320 and 300 flights).",
			wantConf:  model.ConfidenceHigh,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "unknown carrier opens the thin-data sentence",
			carriers: []model.CarrierOnTime{
				{Carrier: testCarrierUA, OnTimeRate: 0.85, Flights: minFlights},
				{Carrier: "", OnTimeRate: 0.50, Flights: 4},
			},
			want:      fmt.Sprintf("UA is the only carrier with enough data on ORD–LAX (85%% on time, medium confidence, %d flights). An unknown carrier (low confidence, 4 flights) has thin data.", minFlights),
			wantConf:  model.ConfidenceMedium,
			wantFirst: testCarrierUA,
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name: "unknown carrier alone on the route",
			carriers: []model.CarrierOnTime{
				{Carrier: "", OnTimeRate: 0.85, Flights: minFlights},
			},
			want:      fmt.Sprintf("An unknown carrier is on time about 85%% of the time on ORD–LAX (medium confidence, %d flights).", minFlights),
			wantConf:  model.ConfidenceMedium,
			wantFirst: "",
			wantStart: windowStart,
			wantEnd:   windowEnd,
		},
		{
			name:      "known route with no flights in range",
			args:      map[string]any{jsonKeyStartDate: testStatsStart, jsonKeyEndDate: testDateJun30},
			carriers:  nil,
			want:      "No flights for ORD–LAX between " + testStatsStart + " and " + testDateJun30 + ", so the confidence is unknown.",
			wantConf:  model.ConfidenceUnknown,
			wantStart: testStatsStart,
			wantEnd:   testDateJun30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotFilter store.RouteStatsFilter

			stub := &storetest.Stub{
				RouteStatsFn: func(_ context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
					gotFilter = filter

					return &model.RouteStats{
						Filters: model.Filters{
							Origin:    filter.Origin,
							Dest:      filter.Dest,
							StartDate: filter.StartDate,
							EndDate:   filter.EndDate,
						},
						Analysis:      model.Analysis{Start: filter.StartDate, End: filter.EndDate, SampleSize: 999},
						Flights:       999,
						CarrierOnTime: tt.carriers,
					}, nil
				},
			}

			args := map[string]any{jsonKeyOrigin: testQueryOrd, jsonKeyDest: "lax"}
			for key, value := range tt.args {
				args[key] = value
			}

			if _, dated := args[jsonKeyStartDate]; !dated {
				stub.RouteLatestFlightDateFn = func(context.Context, string, string) (string, error) {
					return latestCarrierDate, nil
				}
			}

			res := callTool(t, stub, toolCompareCarriers, args)
			if res.IsError {
				t.Fatalf("IsError content=%s", toolText(t, res))
			}

			assertSamplesOnList(t, res, "carriers")

			out := decodeResult[compareCarriersData](t, res)
			if out.Summary != tt.want {
				t.Fatalf("summary = %q", out.Summary)
			}

			if out.Confidence != tt.wantConf {
				t.Fatalf("confidence = %q, want %q", out.Confidence, tt.wantConf)
			}

			if out.Data == nil || out.Data.Stats.Filters.Origin != testCodeORD || out.Data.Stats.Filters.Dest != testCodeLAX || out.Data.Stats.Analysis.Start != tt.wantStart || out.Data.Stats.Analysis.End != tt.wantEnd {
				t.Fatalf("data = %#v", out.Data)
			}

			if len(out.Data.Carriers) != len(tt.carriers) || len(out.Data.Stats.CarrierOnTime) != 0 {
				t.Fatalf("carriers = %d stats carriers = %d", len(out.Data.Carriers), len(out.Data.Stats.CarrierOnTime))
			}

			if out.Data.Stats.Flights != 999 || out.Data.Stats.Analysis.SampleSize != 999 {
				t.Fatalf("stats totals = flights %d sample %d", out.Data.Stats.Flights, out.Data.Stats.Analysis.SampleSize)
			}

			if strings.Contains(toolText(t, res), `"carrier_on_time"`) {
				t.Fatal("carrier_on_time included in compare_carriers")
			}

			if len(tt.carriers) > 0 && out.Data.Carriers[0].Carrier != tt.wantFirst {
				t.Fatalf("first = %q", out.Data.Carriers[0].Carrier)
			}

			if len(tt.carriers) > 0 {
				if out.Data.Carriers[0].SampleSize != tt.carriers[0].Flights || out.Data.Carriers[0].OnTimeRate != tt.carriers[0].OnTimeRate {
					t.Fatalf("first carrier = %#v", out.Data.Carriers[0])
				}
			}

			if gotFilter.Origin != testCodeORD || gotFilter.Dest != testCodeLAX || gotFilter.StartDate != tt.wantStart || gotFilter.EndDate != tt.wantEnd {
				t.Fatalf("filter = %+v", gotFilter)
			}
		})
	}
}

func TestCompareCarriersSampleReasons(t *testing.T) {
	minFlights := store.CarrierStatsMinSampleSize
	stub := &storetest.Stub{
		RouteLatestFlightDateFn: func(context.Context, string, string) (string, error) {
			return latestCarrierDate, nil
		},
		RouteStatsFn: func(_ context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Analysis: model.Analysis{Start: filter.StartDate, End: filter.EndDate},
				CarrierOnTime: []model.CarrierOnTime{
					{Carrier: testCarrierUA, OnTimeRate: 0.80, Flights: minFlights},
					{Carrier: "AA", OnTimeRate: 0.90, Flights: minFlights - 1},
				},
			}, nil
		},
	}

	res := callTool(t, stub, toolCompareCarriers, map[string]any{
		jsonKeyOrigin: testCodeORD,
		jsonKeyDest:   testCodeLAX,
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[compareCarriersData](t, res)
	if len(out.Data.Carriers) != 2 {
		t.Fatalf("carriers = %#v", out.Data.Carriers)
	}

	solid := out.Data.Carriers[0]
	if solid.SampleSize != minFlights || solid.SampleReason != model.SampleReasonSufficient || solid.Confidence != model.ConfidenceMedium {
		t.Fatalf("solid = %#v", solid)
	}

	thin := out.Data.Carriers[1]
	if thin.SampleSize != minFlights-1 || thin.SampleReason != model.SampleReasonInsufficientSample || thin.Confidence != model.ConfidenceLow {
		t.Fatalf("thin = %#v", thin)
	}

	if !strings.Contains(out.Summary, testCarrierUA) || !strings.Contains(out.Summary, "AA") || !strings.Contains(out.Summary, strconv.Itoa(minFlights)) || !strings.Contains(out.Summary, strconv.Itoa(minFlights-1)) {
		t.Fatalf("summary = %q", out.Summary)
	}
}

func TestCompareCarriersErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		latest   func(origin, dest string) (string, error)
		stats    func(store.RouteStatsFilter) (*model.RouteStats, error)
		search   func(string) ([]model.Airport, error)
		wantCode string
		wantText string
		absent   string
		skipCall bool
	}{
		{
			name:     "missing route",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			latest:   func(string, string) (string, error) { return "", store.ErrNotFound },
			search:   func(query string) ([]model.Airport, error) { return exactAirportSearch(context.Background(), query, 1) },
			wantCode: CodeRouteNotFound,
			wantText: testRouteNotFound,
		},
		{
			name: "missing route for explicit dates",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: testStatsStart, jsonKeyEndDate: testDateJun30},
			stats: func(store.RouteStatsFilter) (*model.RouteStats, error) {
				return nil, store.ErrNotFound
			},
			search:   func(query string) ([]model.Airport, error) { return exactAirportSearch(context.Background(), query, 1) },
			wantCode: CodeRouteNotFound,
			wantText: testRouteNotFound,
		},
		{
			name: "unknown origin",
			args: map[string]any{jsonKeyOrigin: testCodeORX, jsonKeyDest: testCodeLAX},
			latest: func(string, string) (string, error) {
				return "", store.ErrNotFound
			},
			search: func(query string) ([]model.Airport, error) {
				if query == testCodeORX {
					return []model.Airport{{IATACode: testCodeORD, Name: testNameOHare, Match: model.AirportMatchCloseCode}}, nil
				}

				return []model.Airport{{IATACode: query, Match: model.AirportMatchExactIATA}}, nil
			},
			wantCode: CodeAirportNotFound,
			wantText: testNameOHare,
		},
		{
			name: "unknown code with no matches",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeZZZ},
			latest: func(string, string) (string, error) {
				return "", store.ErrNotFound
			},
			search: func(query string) ([]model.Airport, error) {
				if query == testCodeZZZ {
					return nil, nil
				}

				return []model.Airport{{IATACode: query, Match: model.AirportMatchExactIATA}}, nil
			},
			wantCode: CodeAirportNotFound,
			wantText: "Use find_airport",
		},
		{
			name: "airport lookup unavailable",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			latest: func(string, string) (string, error) {
				return "", store.ErrNotFound
			},
			search: func(string) ([]model.Airport, error) {
				return nil, errors.New("pq: password authentication failed")
			},
			wantCode: CodeUnavailable,
			wantText: "Airport lookup is unavailable",
			absent:   testLeakedSecret,
		},
		{
			name: "store unavailable",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			latest: func(string, string) (string, error) {
				return "", errors.New("pq: password authentication failed")
			},
			wantCode: CodeUnavailable,
			wantText: msgCarriersUnavailable,
			absent:   testLeakedSecret,
		},
		{
			name: "stats unavailable after latest date",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			latest: func(string, string) (string, error) {
				return latestCarrierDate, nil
			},
			stats: func(store.RouteStatsFilter) (*model.RouteStats, error) {
				return nil, errors.New("pq: password authentication failed")
			},
			wantCode: CodeUnavailable,
			wantText: msgCarriersUnavailable,
			absent:   testLeakedSecret,
		},
		{
			// RouteStats has no stub, so reaching it would panic.
			name: "unparseable latest date",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			latest: func(string, string) (string, error) {
				return "not-a-date", nil
			},
			wantCode: CodeUnavailable,
			wantText: msgCarriersUnavailable,
			absent:   "not-a-date",
		},
		{
			name:     "invalid calendar date",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: "2025-02-31", jsonKeyEndDate: "2025-03-01"},
			wantCode: CodeInvalidInput,
			wantText: "valid date",
		},
		{
			name:     "end before start",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: "2025-06-01", jsonKeyEndDate: "2025-01-01"},
			wantCode: CodeInvalidInput,
			wantText: "on or after",
		},
		{
			name:     "span too long",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: "2024-01-01", jsonKeyEndDate: "2025-01-01"},
			wantCode: CodeInvalidInput,
			wantText: "366",
		},
		{
			name:     "only one date",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: testStatsStart},
			wantCode: CodeInvalidInput,
			wantText: msgDatePair,
		},
		{
			name:     "schema rejects a bad date",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: "2025/01/01", jsonKeyEndDate: testStatsEnd},
			wantText: sdkValidatingArguments,
			skipCall: true,
		},
		{
			name:     "schema rejects a short origin",
			args:     map[string]any{jsonKeyOrigin: "OR", jsonKeyDest: testCodeLAX},
			wantText: sdkValidatingArguments,
			skipCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &storetest.Stub{}
			if tt.latest != nil {
				stub.RouteLatestFlightDateFn = func(_ context.Context, origin, dest string) (string, error) {
					return tt.latest(origin, dest)
				}
			}

			if tt.stats != nil {
				stub.RouteStatsFn = func(_ context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
					return tt.stats(filter)
				}
			}

			if tt.search != nil {
				stub.SearchAirportsFn = func(_ context.Context, query string, _ int) ([]model.Airport, error) {
					return tt.search(query)
				}
			}

			res := callTool(t, stub, toolCompareCarriers, tt.args)
			if !res.IsError {
				t.Fatalf("expected error, content=%s", toolText(t, res))
			}

			text := toolText(t, res)
			if !strings.Contains(text, tt.wantText) {
				t.Fatalf("text = %s", text)
			}

			if tt.absent != "" && strings.Contains(text, tt.absent) {
				t.Fatalf("leaked %q in %s", tt.absent, text)
			}

			if tt.skipCall {
				return
			}

			assertNoSampleFields(t, res)

			out := decodeResult[compareCarriersData](t, res)
			if out.Error == nil || out.Error.Code != tt.wantCode || out.Data != nil {
				t.Fatalf("error = %#v", out.Error)
			}
		})
	}
}

func TestCompareCarriersDirectInvalidInput(t *testing.T) {
	res, out, err := compareCarriers(context.Background(), &storetest.Stub{}, testLogger(), compareCarriersInput{
		Origin: testPaddedOrd,
		Dest:   "",
	})
	if err != nil {
		t.Fatalf("compareCarriers: %v", err)
	}

	if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeInvalidInput || out.Error.Message != msgRoute {
		t.Fatalf("error = %#v", out.Error)
	}
}

func TestCompareCarriersInputSchema(t *testing.T) {
	schema := compareCarriersInputSchema()
	assertCodeSchema(t, schema.Properties[jsonKeyOrigin], 3)
	assertCodeSchema(t, schema.Properties[jsonKeyDest], 3)

	start := schema.Properties[jsonKeyStartDate]
	if start == nil || start.MinLength == nil || *start.MinLength != isoDateLen || start.MaxLength == nil || *start.MaxLength != isoDateLen || start.Pattern != patternISODate {
		t.Fatalf("start_date schema = %#v", start)
	}

	end := schema.Properties[jsonKeyEndDate]
	if end == nil || end.Pattern != patternISODate {
		t.Fatalf("end_date schema = %#v", end)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX}); err != nil {
		t.Fatalf("valid args: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: testStatsStart, jsonKeyEndDate: testStatsEnd}); err != nil {
		t.Fatalf("valid dates: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyStartDate: "01-02-2025", jsonKeyEndDate: testStatsEnd}); err == nil {
		t.Fatal("expected date pattern error")
	}
}
