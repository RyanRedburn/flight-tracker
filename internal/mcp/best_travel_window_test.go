package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBestTravelWindowSummaries(t *testing.T) {
	tests := []struct {
		name       string
		carrier    string
		months     []model.TravelWindowMonthBucket
		want       string
		wantConf   string
		wantMonths int
		firstMonth int
	}{
		{
			name: "later month beats the first",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.70, 100, model.ConfidenceHigh, model.ReliabilityTypical),
				monthBucket(7, 0.90, 110, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "July is more reliable than January on ORD–LAX (90% vs 70% on time, high confidence, 110 and 100 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "exact tie names both months",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.80, 100, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.80, 90, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "January and February are about the same on ORD–LAX (80% on time, high confidence, 100 and 90 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "near tie names both months",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.82, 100, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.80, 90, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "January and February are about the same on ORD–LAX (82% and 80% on time, high confidence, 100 and 90 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "steady slide names every month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 120, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.83, 110, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(3, 0.81, 100, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "January is most reliable on ORD–LAX (85% on time, high confidence, 120 flights), then February (83%, high confidence, 110 flights) and March (81%, high confidence, 100 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "two lead and one trails",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 120, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.84, 110, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(7, 0.70, 90, model.ConfidenceHigh, model.ReliabilityTypical),
			},
			want:       "January and February are about the same and more reliable than July on ORD–LAX (85% and 84% vs 70% on time, high confidence, 120, 110 and 90 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "tied best then a worse month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 100, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.85, 90, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(7, 0.70, 80, model.ConfidenceHigh, model.ReliabilityTypical),
			},
			want:       "January and February are about the same and more reliable than July on ORD–LAX (85% vs 70% on time, high confidence, 100, 90 and 80 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "one solid month and every thin month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 80, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(3, 0.95, 12, model.ConfidenceLow, model.ReliabilityUnknown),
				monthBucket(7, 0.40, 8, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "January is the only month with enough data on ORD–LAX (85% on time, high confidence, 80 flights). March (low confidence, 12 flights) and July (low confidence, 8 flights) have thin data.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "every thin month is named",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.90, 12, model.ConfidenceLow, model.ReliabilityUnknown),
				monthBucket(3, 0.50, 8, model.ConfidenceLow, model.ReliabilityUnknown),
				monthBucket(7, 0.20, 4, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "No month on ORD–LAX has enough data to compare. January (low confidence, 12 flights), March (low confidence, 8 flights) and July (low confidence, 4 flights) have thin data.",
			wantConf:   model.ConfidenceLow,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "one flight stays singular",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 1, 1, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "No month on ORD–LAX has enough data to compare. January (low confidence, 1 flight) has thin data.",
			wantConf:   model.ConfidenceLow,
			wantMonths: 1,
			firstMonth: 1,
		},
		{
			name: "unknown and thin months are both named",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.50, 2, "", model.ReliabilityUnknown),
				monthBucket(7, 0.40, 5, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "No month on ORD–LAX has enough data to compare. July (low confidence, 5 flights) has thin data. January (unknown confidence, 2 flights) has no reliable on-time rate.",
			wantConf:   model.ConfidenceUnknown,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name:       "route exists but no months",
			months:     nil,
			want:       "No month-of-year history for ORD–LAX between 2025-01-01 and 2026-01-01, so the confidence is unknown.",
			wantConf:   model.ConfidenceUnknown,
			wantMonths: 0,
		},
		{
			name:    "carrier is part of the route",
			carrier: "ua",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 80, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(7, 0.60, 70, model.ConfidenceHigh, model.ReliabilityUnreliable),
			},
			want:       "January is more reliable than July on UA ORD–LAX (85% vs 60% on time, high confidence, 80 and 70 flights).",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 2,
			firstMonth: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotFilter store.RouteTravelWindowsFilter

			stub := &storetest.Stub{
				RouteTravelWindowsFn: func(_ context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
					gotFilter = filter

					return testWindows(tt.months), nil
				},
			}

			args := map[string]any{
				jsonKeyOrigin: testQueryOrd,
				jsonKeyDest:   "lax",
			}
			if tt.carrier != "" {
				args[jsonKeyCarrier] = tt.carrier
			}

			res := callTool(t, stub, toolBestTravelWindow, args)
			if res.IsError {
				t.Fatalf("IsError content=%s", toolText(t, res))
			}

			assertSamplesOnList(t, res, "months")

			out := decodeResult[bestTravelWindowData](t, res)
			if out.Summary != tt.want {
				t.Fatalf("summary = %q", out.Summary)
			}

			if out.Confidence != tt.wantConf {
				t.Fatalf("confidence = %q, want %q", out.Confidence, tt.wantConf)
			}

			if out.Data == nil || out.Data.Origin != testCodeORD || out.Data.Dest != testCodeLAX || out.Data.Start != testStatsStart || out.Data.End != testStatsEnd {
				t.Fatalf("data = %#v", out.Data)
			}

			if tt.carrier == "" && out.Data.Carrier != "" {
				t.Fatalf("carrier = %q", out.Data.Carrier)
			}

			if tt.carrier != "" && out.Data.Carrier != testCarrierUA {
				t.Fatalf("carrier = %q", out.Data.Carrier)
			}

			if len(out.Data.Months) != tt.wantMonths {
				t.Fatalf("months = %d", len(out.Data.Months))
			}

			if tt.wantMonths > 0 && out.Data.Months[0].Month != tt.firstMonth {
				t.Fatalf("first month = %d", out.Data.Months[0].Month)
			}

			if gotFilter.Origin != testCodeORD || gotFilter.Dest != testCodeLAX || gotFilter.Carrier != out.Data.Carrier {
				t.Fatalf("filter = %+v", gotFilter)
			}

			text := toolText(t, res)
			for _, absent := range []string{"424242", "0.13", "0.17", "by_hour", "by_day_of_week", "day_of_week"} {
				if strings.Contains(text, absent) {
					t.Errorf("result included %q", absent)
				}
			}
		})
	}
}

func TestBestTravelWindowSampleReasons(t *testing.T) {
	stub := &storetest.Stub{
		RouteTravelWindowsFn: func(context.Context, store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
			return testWindows([]model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 80, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.90, 4, model.ConfidenceLow, model.ReliabilityUnknown),
				monthBucket(3, 0.10, 2, "", model.ReliabilityUnknown),
			}), nil
		},
	}

	res := callTool(t, stub, toolBestTravelWindow, map[string]any{
		jsonKeyOrigin: testCodeORD,
		jsonKeyDest:   testCodeLAX,
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[bestTravelWindowData](t, res)
	if len(out.Data.Months) != 3 {
		t.Fatalf("months = %#v", out.Data.Months)
	}

	if out.Data.Months[0].SampleSize != 80 || out.Data.Months[0].SampleReason != model.SampleReasonSufficient || out.Data.Months[0].Confidence != model.ConfidenceHigh {
		t.Fatalf("solid = %#v", out.Data.Months[0])
	}

	if out.Data.Months[1].SampleSize != 4 || out.Data.Months[1].SampleReason != model.SampleReasonInsufficientSample || out.Data.Months[1].Confidence != model.ConfidenceLow {
		t.Fatalf("thin = %#v", out.Data.Months[1])
	}

	if out.Data.Months[2].SampleSize != 2 || out.Data.Months[2].SampleReason != model.SampleReasonEmptySample || out.Data.Months[2].Confidence != model.ConfidenceUnknown {
		t.Fatalf("unknown = %#v", out.Data.Months[2])
	}
}

func TestBestTravelWindowErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		windows  func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error)
		search   func(string) ([]model.Airport, error)
		wantCode string
		wantText string
		absent   string
		skipCall bool
	}{
		{
			name:     "missing route",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			windows:  func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) { return nil, store.ErrNotFound },
			search:   func(query string) ([]model.Airport, error) { return exactAirportSearch(context.Background(), query, 1) },
			wantCode: CodeRouteNotFound,
			wantText: "No history for ORD–LAX. Check the airports.",
		},
		{
			name:     "missing carrier route",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA},
			windows:  func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) { return nil, store.ErrNotFound },
			search:   func(query string) ([]model.Airport, error) { return exactAirportSearch(context.Background(), query, 1) },
			wantCode: CodeRouteNotFound,
			wantText: "No history for UA ORD–LAX. Check the airports and carrier.",
		},
		{
			name: "unknown origin",
			args: map[string]any{jsonKeyOrigin: testCodeORX, jsonKeyDest: testCodeLAX},
			windows: func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
				return nil, store.ErrNotFound
			},
			search: func(query string) ([]model.Airport, error) {
				if query == testCodeORX {
					return []model.Airport{{IATACode: testCodeORD, Name: testNameOHare, Match: model.AirportMatchCloseCode}}, nil
				}

				return []model.Airport{{IATACode: query, Match: model.AirportMatchExactIATA}}, nil
			},
			wantCode: CodeAirportNotFound,
			wantText: testCodeORD,
		},
		{
			name: "unknown destination",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeZZZ},
			windows: func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
				return nil, store.ErrNotFound
			},
			search: func(query string) ([]model.Airport, error) {
				if query == testCodeZZZ {
					return nil, nil
				}

				return []model.Airport{{IATACode: query, Match: model.AirportMatchExactIATA}}, nil
			},
			wantCode: CodeAirportNotFound,
			wantText: testCodeZZZ,
		},
		{
			name: "airport lookup unavailable",
			args: map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX},
			windows: func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
				return nil, store.ErrNotFound
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
			windows: func(store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
				return nil, errors.New("pq: password authentication failed")
			},
			wantCode: CodeUnavailable,
			wantText: msgTravelWindowsUnavailable,
			absent:   testLeakedSecret,
		},
		{
			name:     "schema rejects a long origin",
			args:     map[string]any{jsonKeyOrigin: "ORDX", jsonKeyDest: testCodeLAX},
			wantText: sdkValidatingArguments,
			skipCall: true,
		},
		{
			name:     "schema rejects a non-letter code",
			args:     map[string]any{jsonKeyOrigin: "OR1", jsonKeyDest: testCodeLAX},
			wantText: sdkValidatingArguments,
			skipCall: true,
		},
		{
			name:     "schema rejects a long carrier",
			args:     map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: "UAL"},
			wantText: sdkValidatingArguments,
			skipCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &storetest.Stub{}
			if tt.windows != nil {
				stub.RouteTravelWindowsFn = func(_ context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
					return tt.windows(filter)
				}
			}

			if tt.search != nil {
				stub.SearchAirportsFn = func(_ context.Context, query string, _ int) ([]model.Airport, error) {
					return tt.search(query)
				}
			}

			res := callTool(t, stub, toolBestTravelWindow, tt.args)
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

			out := decodeResult[bestTravelWindowData](t, res)
			if out.Error == nil || out.Error.Code != tt.wantCode || out.Data != nil {
				t.Fatalf("error = %#v data=%#v", out.Error, out.Data)
			}

			if out.Summary != out.Error.Message {
				t.Fatalf("summary = %q message = %q", out.Summary, out.Error.Message)
			}
		})
	}
}

func TestBestTravelWindowDirectInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   bestTravelWindowInput
		want string
	}{
		{name: "blank route", in: bestTravelWindowInput{Origin: "  ", Dest: " "}, want: msgRoute},
		{name: "origin only", in: bestTravelWindowInput{Origin: testCodeORD}, want: msgRoute},
		{name: "short origin", in: bestTravelWindowInput{Origin: "OR", Dest: testCodeLAX}, want: "origin must be a 3-letter IATA airport code."},
		{name: "short dest", in: bestTravelWindowInput{Origin: testCodeORD, Dest: "LA"}, want: "dest must be a 3-letter IATA airport code."},
		{name: "short carrier", in: bestTravelWindowInput{Origin: testCodeORD, Dest: testCodeLAX, Carrier: "U"}, want: msgCarrierCode},
		{name: "padded codes", in: bestTravelWindowInput{Origin: testPaddedOrd, Dest: testPaddedLax, Carrier: testPaddedUA}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got store.RouteTravelWindowsFilter

			stub := &storetest.Stub{
				RouteTravelWindowsFn: func(_ context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
					got = filter

					return testWindows([]model.TravelWindowMonthBucket{
						monthBucket(1, 0.8, 80, model.ConfidenceHigh, model.ReliabilityReliable),
					}), nil
				},
			}

			res, out, err := bestTravelWindow(context.Background(), stub, testLogger(), tt.in)
			if err != nil {
				t.Fatalf("bestTravelWindow: %v", err)
			}

			if tt.want == "" {
				if res == nil || res.IsError || got.Origin != testCodeORD || got.Dest != testCodeLAX || got.Carrier != testCarrierUA {
					t.Fatalf("filter = %+v error = %#v", got, out.Error)
				}

				return
			}

			if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeInvalidInput || out.Error.Message != tt.want {
				t.Fatalf("error = %#v", out.Error)
			}
		})
	}
}

func TestBestTravelWindowInputSchema(t *testing.T) {
	schema := bestTravelWindowInputSchema()
	assertCodeSchema(t, schema.Properties[jsonKeyOrigin], 3)
	assertCodeSchema(t, schema.Properties[jsonKeyDest], 3)
	assertCodeSchema(t, schema.Properties[jsonKeyCarrier], 2)

	if schema.Properties[jsonKeyOrigin].Pattern != patternIATACode || schema.Properties[jsonKeyCarrier].Pattern != patternCarrierCode {
		t.Fatalf("patterns = origin %q carrier %q", schema.Properties[jsonKeyOrigin].Pattern, schema.Properties[jsonKeyCarrier].Pattern)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX}); err != nil {
		t.Fatalf("valid args: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: "B6"}); err != nil {
		t.Fatalf("alphanumeric carrier: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyOrigin: "OR1", jsonKeyDest: testCodeLAX}); err == nil {
		t.Fatal("expected pattern error")
	}
}

func monthBucket(month int, rate float64, flights int, confidence, reliability string) model.TravelWindowMonthBucket {
	return model.TravelWindowMonthBucket{
		Month:       month,
		OnTimeRate:  rate,
		Flights:     flights,
		Confidence:  confidence,
		Reliability: reliability,
	}
}

func testWindows(months []model.TravelWindowMonthBucket) *model.RouteTravelWindows {
	return &model.RouteTravelWindows{
		Analysis: model.Analysis{Start: testStatsStart, End: testStatsEnd, SampleSize: 424242},
		ByMonth:  months,
		ByDayOfWeek: []model.TravelWindowDayBucket{{
			DayOfWeek:   1,
			OnTimeRate:  0.13,
			Flights:     80,
			Confidence:  model.ConfidenceHigh,
			Reliability: model.ReliabilityTypical,
		}},
		ByHour: []model.TravelWindowHourBucket{{
			Hour:        7,
			OnTimeRate:  0.17,
			Flights:     40,
			Confidence:  model.ConfidenceHigh,
			Reliability: model.ReliabilityTypical,
		}},
	}
}

func assertNoSampleFields(t *testing.T, res *mcpsdk.CallToolResult) {
	t.Helper()

	assertTextJSONEqualsStructured(t, res)

	var body map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &body); err != nil {
		t.Fatalf("json: %v\n%s", err, toolText(t, res))
	}

	if _, ok := body["sample_size"]; ok {
		t.Fatal("top-level sample_size")
	}

	if _, ok := body["sample_reason"]; ok {
		t.Fatal("top-level sample_reason")
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		return
	}

	if _, ok := data["sample_size"]; ok {
		t.Fatal("data sample_size")
	}

	if _, ok := data["sample_reason"]; ok {
		t.Fatal("data sample_reason")
	}

	if _, ok := data["analysis"]; ok {
		t.Fatal("data analysis")
	}
}

func assertSamplesOnList(t *testing.T, res *mcpsdk.CallToolResult, listKey string) {
	t.Helper()

	assertNoSampleFields(t, res)

	var body map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &body); err != nil {
		t.Fatal(err)
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", body["data"])
	}

	items, ok := data[listKey].([]any)
	if !ok {
		t.Fatalf("%s = %#v", listKey, data[listKey])
	}

	for i, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("item %d = %#v", i, item)
		}

		if _, ok := row["sample_size"]; !ok {
			t.Fatalf("item %d missing sample_size: %#v", i, row)
		}

		if _, ok := row["sample_reason"]; !ok {
			t.Fatalf("item %d missing sample_reason: %#v", i, row)
		}
	}
}
