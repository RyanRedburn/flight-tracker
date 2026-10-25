package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const (
	testDep0900 = "0900"
	testDep1100 = "1100"
)

func TestRouteReliabilitySummaries(t *testing.T) {
	tests := []struct {
		name           string
		times          []string
		byTime         map[string]model.RouteOutlook
		want           string
		wantConfidence string
	}{
		{
			name:  "one time with data",
			times: []string{"0700"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 30, model.ConfidenceMedium),
			},
			want:           "0700 on UA ORD–LAX is on time about 85% of the time, medium confidence, from 30 flights.",
			wantConfidence: model.ConfidenceMedium,
		},
		{
			name:  "one medium time reports the rate",
			times: []string{"0700"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 40, model.ConfidenceMedium),
			},
			want:           "0700 on UA ORD–LAX is on time about 85% of the time, medium confidence, from 40 flights.",
			wantConfidence: model.ConfidenceMedium,
		},
		{
			name:  "medium confidence times are compared",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 40, model.ConfidenceMedium),
				"1400": reliabilityOutlook(0.60, 40, model.ConfidenceMedium),
			},
			want:           "0700 is more reliable than 1400 on UA ORD–LAX (85% vs 60% on time).",
			wantConfidence: model.ConfidenceMedium,
		},
		{
			name:  "one low time reports the rate",
			times: []string{"0700"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 4, model.ConfidenceLow),
			},
			want:           "0700 on UA ORD–LAX is on time about 85% of the time, low confidence, from 4 flights.",
			wantConfidence: model.ConfidenceLow,
		},
		{
			name:  "one time with no data",
			times: []string{"0700"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0, 0, model.ConfidenceUnknown),
			},
			want:           "0700 on UA ORD–LAX has no data for this day and time.",
			wantConfidence: model.ConfidenceUnknown,
		},
		{
			name:  "three times fully separated",
			times: []string{"0700", "1200", "1800"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.90, 200, model.ConfidenceHigh),
				"1200": reliabilityOutlook(0.70, 200, model.ConfidenceHigh),
				"1800": reliabilityOutlook(0.50, 200, model.ConfidenceHigh),
			},
			want:           "0700 is most reliable on UA ORD–LAX (90% on time), then 1200 (70%) and 1800 (50%).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "two times clear winner",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				"1400": reliabilityOutlook(0.60, 40, model.ConfidenceHigh),
			},
			want:           "0700 is more reliable than 1400 on UA ORD–LAX (85% vs 60% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "two times tie",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.80, 40, model.ConfidenceHigh),
				"1400": reliabilityOutlook(0.80, 36, model.ConfidenceHigh),
			},
			want:           "Both times are too close to call on UA ORD–LAX (80% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "two times near tie",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.82, 40, model.ConfidenceHigh),
				"1400": reliabilityOutlook(0.80, 40, model.ConfidenceHigh),
			},
			want:           "Both times are too close to call on UA ORD–LAX (80–82% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "only one time has data",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 30, model.ConfidenceMedium),
				"1400": reliabilityOutlook(0, 0, model.ConfidenceUnknown),
			},
			want:           "0700 is the only listed time on UA ORD–LAX (85% on time). 1400 has no data.",
			wantConfidence: model.ConfidenceMedium,
		},
		{
			name:  "no times have data",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0, 0, model.ConfidenceUnknown),
				"1400": reliabilityOutlook(0, 0, model.ConfidenceUnknown),
			},
			want:           "No listed time on UA ORD–LAX has data to compare. 0700 and 1400 have no data.",
			wantConfidence: model.ConfidenceUnknown,
		},
		{
			name:  "low confidence time is ranked",
			times: []string{"0700", "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 30, model.ConfidenceMedium),
				"1400": reliabilityOutlook(0.95, 4, model.ConfidenceLow),
			},
			want:           "Both times are too close to call on UA ORD–LAX (85–95% on time).",
			wantConfidence: model.ConfidenceLow,
		},
		{
			name:  "in-between time lists in order",
			times: []string{"0700", "1400", "1800"},
			byTime: map[string]model.RouteOutlook{
				"0700": reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				"1400": reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
				"1800": reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
			},
			want:           "All 3 times are too close to call on UA ORD–LAX (81–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "steady slide lists in order",
			times: []string{"0700", testDep0900, testDep1100, "1400", "1800"},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
				"1400":      reliabilityOutlook(0.79, 40, model.ConfidenceHigh),
				"1800":      reliabilityOutlook(0.77, 40, model.ConfidenceHigh),
			},
			want:           "All 5 times are too close to call on UA ORD–LAX (77–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "two lead one trails",
			times: []string{"0700", testDep0900, testDep1100},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.84, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.80, 40, model.ConfidenceHigh),
			},
			want:           "All 3 times are too close to call on UA ORD–LAX (80–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "best listed first when given out of order",
			times: []string{"0700", testDep0900, testDep1100},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
			},
			want:           "All 3 times are too close to call on UA ORD–LAX (81–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "tied best then in-between times",
			times: []string{"0700", testDep0900, testDep1100, "1800"},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
				"1800":      reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
			},
			want:           "All 4 times are too close to call on UA ORD–LAX (81–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "three tied within the gap stay too close to call",
			times: []string{"0700", testDep0900, testDep1100, "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				"1400":      reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
			},
			want:           "All 4 times are too close to call on UA ORD–LAX (83–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "three tied best then in-between times",
			times: []string{"0700", testDep0900, testDep1100, "1400", "1800"},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				"1400":      reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
				"1800":      reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
			},
			want:           "All 5 times are too close to call on UA ORD–LAX (81–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "rounded percents tie",
			times: []string{"0700", testDep0900, testDep1100, "1800"},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.851, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.849, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
				"1800":      reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
			},
			want:           "All 4 times are too close to call on UA ORD–LAX (81–85% on time).",
			wantConfidence: model.ConfidenceHigh,
		},
		{
			name:  "in-between time with thin data",
			times: []string{"0700", testDep0900, testDep1100, "1400"},
			byTime: map[string]model.RouteOutlook{
				"0700":      reliabilityOutlook(0.85, 40, model.ConfidenceHigh),
				testDep0900: reliabilityOutlook(0.83, 40, model.ConfidenceHigh),
				testDep1100: reliabilityOutlook(0.81, 40, model.ConfidenceHigh),
				"1400":      reliabilityOutlook(0.95, 4, model.ConfidenceLow),
			},
			want:           "All 4 times are too close to call on UA ORD–LAX (81–95% on time).",
			wantConfidence: model.ConfidenceLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var statsFilter store.RouteStatsFilter

			stub := &storetest.Stub{
				RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
					outlook, ok := tt.byTime[filter.DepTime]
					if !ok {
						t.Fatalf("unexpected dep time %s", filter.DepTime)
					}

					outlook.Analysis.Start = testStatsStart
					outlook.Analysis.End = testStatsEnd

					return &outlook, nil
				},
				RouteStatsFn: func(_ context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
					statsFilter = filter

					return &model.RouteStats{
						Flights:  80,
						Analysis: model.Analysis{SampleSize: 80, SampleReason: model.SampleReasonSufficient, Confidence: model.ConfidenceHigh},
					}, nil
				},
			}

			res, out, err := routeReliability(context.Background(), stub, testLogger(), routeReliabilityInput{
				Origin:   testCodeORD,
				Dest:     testCodeLAX,
				Carrier:  testCarrierUA,
				Date:     testDateOct6,
				DepTimes: tt.times,
			})
			if err != nil {
				t.Fatalf("routeReliability: %v", err)
			}

			if res == nil || res.IsError {
				t.Fatalf("error = %#v", out.Error)
			}

			if out.Summary != tt.want {
				t.Fatalf("summary = %q", out.Summary)
			}

			if out.Confidence != tt.wantConfidence {
				t.Fatalf("confidence = %q, want %q", out.Confidence, tt.wantConfidence)
			}

			if statsFilter.StartDate != testStatsStart || statsFilter.EndDate != testStatsEnd || len(statsFilter.DaysOfWeek) != 1 || statsFilter.DaysOfWeek[0] != 2 {
				t.Fatalf("stats filter = %+v", statsFilter)
			}

			if len(out.Data.Departures) != len(tt.times) {
				t.Fatalf("departures = %d", len(out.Data.Departures))
			}

			if out.Data.Departures[0].Filters.DepTime != tt.times[0] || out.Data.Departures[0].Filters.Origin != "" {
				t.Fatalf("departure filters = %+v", out.Data.Departures[0].Filters)
			}
		})
	}
}

func TestRouteReliabilityEmptyStats(t *testing.T) {
	stub := &storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			outlook := reliabilityOutlook(0.5, 4, model.ConfidenceLow)
			outlook.Analysis.Start = testStatsStart
			outlook.Analysis.End = testStatsEnd

			return &outlook, nil
		},
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{Flights: 0, Analysis: model.Analysis{Start: testStatsStart, End: testStatsEnd}}, nil
		},
	}

	res, out, err := routeReliability(context.Background(), stub, testLogger(), routeReliabilityInput{
		Origin:    testCodeORD,
		Dest:      testCodeLAX,
		Carrier:   testCarrierUA,
		DayOfWeek: 2,
		DepTimes:  []string{"0700"},
	})
	if err != nil {
		t.Fatalf("routeReliability: %v", err)
	}

	if res == nil || res.IsError {
		t.Fatalf("error = %#v", out.Error)
	}

	stats := out.Data.Stats
	if stats.Flights != 0 || stats.Analysis.Start != testStatsStart || stats.Analysis.End != testStatsEnd || stats.Analysis.SampleSize != 0 || stats.Analysis.Confidence != "" || stats.Analysis.SampleReason != "" {
		t.Fatalf("stats = %+v", stats)
	}
}

func reliabilityOutlook(rate float64, sample int, confidence string) model.RouteOutlook {
	outlook := model.RouteOutlook{
		OutlookResult: model.OutlookResult{
			Analysis: model.Analysis{
				SampleSize: sample,
				Confidence: confidence,
			},
		},
	}

	switch confidence {
	case model.ConfidenceHigh, model.ConfidenceMedium:
		outlook.Analysis.SampleReason = model.SampleReasonSufficient
		outlook.OnTimeProbability = &rate
	case model.ConfidenceLow:
		outlook.Analysis.SampleReason = model.SampleReasonInsufficientSample
		outlook.OnTimeProbability = &rate
	default:
		outlook.Analysis.SampleReason = model.SampleReasonEmptySample
	}

	return outlook
}

func TestRouteReliabilitySetsDepTimeWindow(t *testing.T) {
	window := 45

	tests := []struct {
		name     string
		minutes  *int
		wantSet  bool
		wantMins int
	}{
		{name: "default window is not caller-set", wantSet: false, wantMins: store.DefaultDepTimeWindowMinutes},
		{name: "explicit window is caller-set", minutes: &window, wantSet: true, wantMins: window},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got store.RouteOutlookFilter

			stub := &storetest.Stub{
				RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
					got = filter

					return &model.RouteOutlook{OutlookResult: model.OutlookResult{
						Analysis: model.Analysis{Start: testStatsStart, End: testStatsEnd, SampleSize: 40, Confidence: model.ConfidenceMedium},
					}}, nil
				},
				RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
					return &model.RouteStats{
						Flights:  40,
						Analysis: model.Analysis{Start: testStatsStart, End: testStatsEnd, SampleSize: 40},
					}, nil
				},
			}

			res, out, err := routeReliability(context.Background(), stub, testLogger(), routeReliabilityInput{
				Origin:               testCodeORD,
				Dest:                 testCodeLAX,
				Carrier:              testCarrierUA,
				Date:                 testDateOct6,
				DepTimes:             []string{"0700"},
				DepTimeWindowMinutes: tt.minutes,
			})
			if err != nil {
				t.Fatalf("routeReliability: %v", err)
			}

			if res == nil || res.IsError {
				t.Fatalf("error = %#v", out.Error)
			}

			if got.DepTimeWindowSet != tt.wantSet || got.DepTimeWindowMinutes != tt.wantMins {
				t.Fatalf("window set=%v minutes=%d", got.DepTimeWindowSet, got.DepTimeWindowMinutes)
			}
		})
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
		jsonKeyFlightNumber: "0200",
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

func TestRouteReliabilityFlightNumberNoNearbyRoute(t *testing.T) {
	stub := &storetest.Stub{
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return []model.FlightNumberRoute{
				{Origin: testCodeORD, Dest: testCodeLAX, Flights: 40, UsualDepTime: "0700"},
				{Origin: testCodeORD, Dest: testCodeSFO, Flights: 30, UsualDepTime: "1800"},
			}, nil
		},
	}

	res := callTool(t, stub, toolRouteReliability, map[string]any{
		jsonKeyCarrier:      testCarrierUA,
		jsonKeyFlightNumber: "1234",
		jsonKeyDayOfWeek:    2,
		jsonKeyDepTimes:     []string{testDep0900},
	})
	if !res.IsError {
		t.Fatal("expected error")
	}

	out := decodeResult[routeReliabilityData](t, res)

	want := "No UA 1234 route leaves near " + testDep0900 + "; recent routes are listed. Pick origin and destination."
	if out.Error == nil || out.Error.Code != CodeFlightNumberAmbiguous || out.Error.Message != want {
		t.Fatalf("error = %#v", out.Error)
	}

	if len(out.Error.FlightRoutes) != 2 {
		t.Fatalf("flight_routes = %#v", out.Error.FlightRoutes)
	}
}

func TestRouteReliabilityFlightNumberAmbiguousDepTimes(t *testing.T) {
	stub := &storetest.Stub{
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return []model.FlightNumberRoute{
				{Origin: testCodeORD, Dest: testCodeLAX, Flights: 40, UsualDepTime: "0700"},
				{Origin: testCodeORD, Dest: testCodeSFO, Flights: 30, UsualDepTime: "1800"},
			}, nil
		},
	}

	res := callTool(t, stub, toolRouteReliability, map[string]any{
		jsonKeyCarrier:      testCarrierUA,
		jsonKeyFlightNumber: "200",
		jsonKeyDayOfWeek:    2,
		jsonKeyDepTimes:     []string{"0700", "1800"},
	})
	if !res.IsError {
		t.Fatal("expected error")
	}

	out := decodeResult[routeReliabilityData](t, res)

	want := "UA 200 has more than one route near 0700 and 1800. Pick origin and destination."
	if out.Error == nil || out.Error.Code != CodeFlightNumberAmbiguous || out.Error.Message != want {
		t.Fatalf("error = %#v", out.Error)
	}

	if len(out.Error.FlightRoutes) != 2 {
		t.Fatalf("flight_routes = %#v", out.Error.FlightRoutes)
	}
}

func TestRouteReliabilityFlightNumberMatchesLaterDepTime(t *testing.T) {
	var got []store.RouteOutlookFilter

	stub := &storetest.Stub{
		SearchAirportsFn: exactAirportSearch,
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return []model.FlightNumberRoute{
				{Origin: testCodeORD, Dest: testCodeLAX, Flights: 40, UsualDepTime: "0700"},
				{Origin: testCodeORD, Dest: testCodeSFO, Flights: 30, UsualDepTime: "1800"},
			}, nil
		},
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			got = append(got, filter)
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
		jsonKeyDepTimes:     []string{"0900", "1805"},
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	if len(got) != 2 || got[0].Origin != testCodeORD || got[0].Dest != testCodeSFO || got[0].DepTime != "0900" || got[1].DepTime != "1805" || got[1].Dest != testCodeSFO {
		t.Fatalf("filters = %+v", got)
	}
}

func TestRouteReliabilityRouteNotFound(t *testing.T) {
	stub := &storetest.Stub{
		SearchAirportsFn: exactAirportSearch,
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return nil, store.ErrNotFound
		},
	}

	res := callTool(t, stub, toolRouteReliability, map[string]any{
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
}

func TestRouteReliabilityUnknownAirport(t *testing.T) {
	stub := &storetest.Stub{
		SearchAirportsFn: func(_ context.Context, query string, _ int) ([]model.Airport, error) {
			if query == testCodeORX {
				return []model.Airport{{IATACode: testCodeORD, Name: testNameOHare, Match: model.AirportMatchCloseCode}}, nil
			}

			return []model.Airport{{IATACode: query, Match: model.AirportMatchExactIATA}}, nil
		},
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return nil, store.ErrNotFound
		},
	}

	res, out, err := routeReliability(context.Background(), stub, testLogger(), routeReliabilityInput{
		Origin:   testCodeORX,
		Dest:     testCodeLAX,
		Carrier:  testCarrierUA,
		Date:     testDateOct6,
		DepTimes: []string{"0700"},
	})
	if err != nil {
		t.Fatalf("routeReliability: %v", err)
	}

	if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeAirportNotFound {
		t.Fatalf("error = %#v", out.Error)
	}

	if len(out.Error.Matches) != 1 || out.Error.Matches[0].IATACode != testCodeORD {
		t.Fatalf("matches = %#v", out.Error.Matches)
	}
}

func TestRouteReliabilityFlightNumberSkipsAirportSearch(t *testing.T) {
	stub := &storetest.Stub{
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return []model.FlightNumberRoute{{Origin: testCodeORD, Dest: testCodeLAX, Flights: 10, UsualDepTime: "0700"}}, nil
		},
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return nil, store.ErrNotFound
		},
	}

	res, out, err := routeReliability(context.Background(), stub, testLogger(), routeReliabilityInput{
		Carrier:      testCarrierUA,
		FlightNumber: "200",
		Date:         testDateOct6,
		DepTimes:     []string{"0700"},
	})
	if err != nil {
		t.Fatalf("routeReliability: %v", err)
	}

	if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeRouteNotFound {
		t.Fatalf("error = %#v", out.Error)
	}
}

func TestRouteReliabilityDuplicateDepTimes(t *testing.T) {
	res, out, err := routeReliability(context.Background(), &storetest.Stub{}, testLogger(), routeReliabilityInput{
		Origin:   testCodeORD,
		Dest:     testCodeLAX,
		Carrier:  testCarrierUA,
		Date:     testDateOct6,
		DepTimes: []string{"0700", "0700"},
	})
	if err != nil {
		t.Fatalf("routeReliability: %v", err)
	}

	if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeInvalidInput {
		t.Fatalf("error = %#v", out.Error)
	}

	if out.Error.Message != "Each dep_times entry must be a different time." {
		t.Fatalf("message = %q", out.Error.Message)
	}
}

func TestRouteReliabilityValidationMessages(t *testing.T) {
	window := 0
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
		want string
	}{
		{
			name: "neither date nor day",
			edit: func(in *routeReliabilityInput) { in.Date = "" },
			want: msgOutlookSchedule,
		},
		{
			name: "both date and day",
			edit: func(in *routeReliabilityInput) { in.DayOfWeek = 2 },
			want: msgOutlookSchedule,
		},
		{
			name: "bad date",
			edit: func(in *routeReliabilityInput) { in.Date = "2026-13-01" },
			want: "date must be a valid date (YYYY-MM-DD).",
		},
		{
			name: "bad weekday",
			edit: func(in *routeReliabilityInput) {
				in.Date = ""
				in.DayOfWeek = 8
			},
			want: "day_of_week must be from 1 to 7.",
		},
		{
			name: "bad window",
			edit: func(in *routeReliabilityInput) { in.DepTimeWindowMinutes = &window },
			want: "dep_time_window_minutes must be from 1 to 120.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.edit(&in)

			res, out, err := routeReliability(context.Background(), &storetest.Stub{}, testLogger(), in)
			if err != nil {
				t.Fatalf("routeReliability: %v", err)
			}

			if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeInvalidInput || out.Error.Message != tt.want {
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

	stub := &storetest.Stub{}

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
	if flightNumber == nil || flightNumber.MaxLength == nil || *flightNumber.MaxLength != store.FlightNumberRawMaxLen || flightNumber.Pattern != store.FlightNumberPattern {
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
		jsonKeyDate: testDateOct6, jsonKeyDepTimes: []string{"0700", "0800", "0930", "1000", "1100", "1200"},
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

func TestRouteDepartureOutlookJSON(t *testing.T) {
	rate := 0.85

	body, err := json.Marshal(newRouteDepartureOutlook(model.RouteOutlook{
		OutlookResult: model.OutlookResult{
			Analysis:          model.Analysis{SampleSize: 30, Confidence: model.ConfidenceMedium},
			OnTimeProbability: &rate,
			Connection:        model.ConnectionGuidance{DomesticToDomestic: model.NewConnectionMinutes(45)},
		},
	}, "0700", 30))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := payload["connection"]; ok {
		t.Fatalf("connection present in %s", body)
	}

	filters, ok := payload["filters"].(map[string]any)
	if !ok || filters["dep_time"] != "0700" {
		t.Fatalf("filters = %#v", payload["filters"])
	}

	if _, ok := filters["origin"]; ok {
		t.Fatalf("origin repeated in %s", body)
	}
}
