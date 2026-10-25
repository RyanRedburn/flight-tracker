package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
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
			want:       "July is the most reliable month for ORD–LAX (90% on time), and January is the least reliable (70%). High confidence, 100–110 flights each.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "medium months are compared",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.80, 120, model.ConfidenceMedium, model.ReliabilityReliable),
				monthBucket(2, 0.80, 110, model.ConfidenceMedium, model.ReliabilityReliable),
			},
			want:       "Both months are too close to call for ORD–LAX (80% on time), medium confidence, 110–120 flights each.",
			wantConf:   model.ConfidenceMedium,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "result confidence is the weaker month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.90, 400, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(7, 0.60, 120, model.ConfidenceMedium, model.ReliabilityUnreliable),
			},
			want:       "January is the most reliable month for ORD–LAX (90% on time), and July is the least reliable (60%). Medium confidence, 120–400 flights each.",
			wantConf:   model.ConfidenceMedium,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "exact tie names both months",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.80, 100, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.80, 90, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "Both months are too close to call for ORD–LAX (80% on time), high confidence, 90–100 flights each.",
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
			want:       "Both months are too close to call for ORD–LAX (80–82% on time), high confidence, 90–100 flights each.",
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
			want:       "All 3 months are too close to call for ORD–LAX (81–85% on time), high confidence, 100–120 flights each.",
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
			want:       "January and February are the most reliable months for ORD–LAX (85% and 84% on time), and July is the least reliable (70%). High confidence, 90–120 flights each.",
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
			want:       "February and January are the most reliable months for ORD–LAX (85% on time), and July is the least reliable (70%). High confidence, 80–100 flights each.",
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
			want:       "March and January are the most reliable months for ORD–LAX (95% and 85% on time), and July is the least reliable (40%). Low confidence, 8–80 flights each.",
			wantConf:   model.ConfidenceLow,
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
			want:       "January and March are the most reliable months for ORD–LAX (90% and 50% on time), and July is the least reliable (20%). Low confidence, 4–12 flights each.",
			wantConf:   model.ConfidenceLow,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "one flight stays singular",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 1, 1, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "January is the only month for ORD–LAX (100% on time). Low confidence, 1 flight.",
			wantConf:   model.ConfidenceLow,
			wantMonths: 1,
			firstMonth: 1,
		},
		{
			name: "unknown and thin months are both named",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.50, 2, model.ConfidenceUnknown, model.ReliabilityUnknown),
				monthBucket(7, 0.40, 5, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "Both months are too close to call for ORD–LAX (40–50% on time), unknown confidence, 2–5 flights each.",
			wantConf:   model.ConfidenceUnknown,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "tied best and worst leave the middle month out",
			months: []model.TravelWindowMonthBucket{
				monthBucket(5, 0.86, 500, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(6, 0.80, 650, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(8, 0.72, 440, model.ConfidenceHigh, model.ReliabilityTypical),
				monthBucket(9, 0.86, 435, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "May and September are the most reliable months for ORD–LAX (86% on time), and August is the least reliable (72%). High confidence, 435–650 flights each.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 4,
			firstMonth: 5,
		},
		{
			name:    "carrier route names tied best months",
			carrier: "ua",
			months: []model.TravelWindowMonthBucket{
				monthBucket(5, 0.86, 650, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(6, 0.80, 500, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(8, 0.72, 600, model.ConfidenceHigh, model.ReliabilityTypical),
				monthBucket(9, 0.86, 435, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "May and September are the most reliable months for UA ORD–LAX (86% on time), and August is the least reliable (72%). High confidence, 435–650 flights each.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 4,
			firstMonth: 5,
		},
		{
			name: "tied worst months are both named",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.90, 200, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.80, 150, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(3, 0.70, 100, model.ConfidenceHigh, model.ReliabilityTypical),
				monthBucket(4, 0.70, 180, model.ConfidenceHigh, model.ReliabilityTypical),
			},
			want:       "January is the most reliable month for ORD–LAX (90% on time), and April and March are the least reliable (70%). High confidence, 100–200 flights each.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 4,
			firstMonth: 1,
		},
		{
			name: "wilson leader separates the lower month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.88, 110, model.ConfidenceMedium, model.ReliabilityReliable),
				monthBucket(2, 0.85, 3000, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(3, 0.82, 3000, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "January and February are the most reliable months for ORD–LAX (88% and 85% on time), and March is the least reliable (82%). Medium confidence, 110–3000 flights each.",
			wantConf:   model.ConfidenceMedium,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "one solid month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(5, 0.86, 435, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "May is the only month for ORD–LAX (86% on time). High confidence, 435 flights.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 1,
			firstMonth: 5,
		},
		{
			name: "every solid month has the same flight count",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.80, 100, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.80, 100, model.ConfidenceHigh, model.ReliabilityReliable),
			},
			want:       "Both months are too close to call for ORD–LAX (80% on time), high confidence, 100 flights each.",
			wantConf:   model.ConfidenceHigh,
			wantMonths: 2,
			firstMonth: 1,
		},
		{
			name: "solid months with a thin month between them",
			months: []model.TravelWindowMonthBucket{
				monthBucket(1, 0.85, 120, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(2, 0.80, 100, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(3, 0.95, 12, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "All 3 months are too close to call for ORD–LAX (80–95% on time), low confidence, 12–120 flights each.",
			wantConf:   model.ConfidenceLow,
			wantMonths: 3,
			firstMonth: 1,
		},
		{
			name: "near tie among three solid months with a thin month",
			months: []model.TravelWindowMonthBucket{
				monthBucket(4, 0.84, 300, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(5, 0.86, 320, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(6, 0.85, 300, model.ConfidenceHigh, model.ReliabilityReliable),
				monthBucket(12, 0.60, 20, model.ConfidenceLow, model.ReliabilityUnknown),
			},
			want:       "May, June and April are the most reliable months for ORD–LAX (86%, 85% and 84% on time), and December is the least reliable (60%). Low confidence, 20–320 flights each.",
			wantConf:   model.ConfidenceLow,
			wantMonths: 4,
			firstMonth: 4,
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
			want:       "January is the most reliable month for UA ORD–LAX (85% on time), and July is the least reliable (60%). High confidence, 70–80 flights each.",
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

					windows := testWindows(tt.months)
					windows.Filters = model.Filters{Origin: filter.Origin, Dest: filter.Dest, Carrier: filter.Carrier}

					return windows, nil
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

			if tt.wantMonths > 0 {
				assertSamplesOnList(t, res, "by_month")
			} else {
				assertNoSampleFields(t, res)
			}

			out := decodeResult[bestTravelWindowData](t, res)
			if out.Summary != tt.want {
				t.Fatalf("summary = %q", out.Summary)
			}

			if out.Confidence != tt.wantConf {
				t.Fatalf("confidence = %q, want %q", out.Confidence, tt.wantConf)
			}

			if out.Data == nil || out.Data.Filters.Origin != testCodeORD || out.Data.Filters.Dest != testCodeLAX || out.Data.Start != testStatsStart || out.Data.End != testStatsEnd {
				t.Fatalf("data = %#v", out.Data)
			}

			if tt.carrier == "" && out.Data.Filters.Carrier != "" {
				t.Fatalf("carrier = %q", out.Data.Filters.Carrier)
			}

			if tt.carrier != "" && out.Data.Filters.Carrier != testCarrierUA {
				t.Fatalf("carrier = %q", out.Data.Filters.Carrier)
			}

			if len(out.Data.ByMonth) != tt.wantMonths {
				t.Fatalf("months = %d", len(out.Data.ByMonth))
			}

			if tt.wantMonths > 0 && (out.Data.ByMonth[0].Month != tt.firstMonth || out.Data.ByMonth[0].SampleSize != tt.months[0].Flights || out.Data.ByMonth[0].OnTimeRate != tt.months[0].OnTimeRate || out.Data.ByMonth[0].Reliability != tt.months[0].Reliability || out.Data.ByMonth[0].Confidence != tt.months[0].Confidence) {
				t.Fatalf("first month = %#v", out.Data.ByMonth[0])
			}

			if listsOverlap(out.Data.BestMonths, out.Data.WorstMonths) {
				t.Fatalf("best %+v overlaps worst %+v", out.Data.BestMonths, out.Data.WorstMonths)
			}

			if tt.wantMonths > 0 && out.Data.MonthComparison == "" {
				t.Fatal("missing month_comparison")
			}

			if (out.Data.MonthComparison == model.ComparisonTooCloseToCall || out.Data.MonthComparison == model.ComparisonSingle) && (len(out.Data.BestMonths) != 0 || len(out.Data.WorstMonths) != 0) {
				t.Fatalf("comparison %q best %+v worst %+v", out.Data.MonthComparison, out.Data.BestMonths, out.Data.WorstMonths)
			}

			if gotFilter.Origin != testCodeORD || gotFilter.Dest != testCodeLAX || gotFilter.Carrier != out.Data.Filters.Carrier {
				t.Fatalf("filter = %+v", gotFilter)
			}

			text := toolText(t, res)
			for _, absent := range []string{"4242", "0.13", "0.17", "by_hour", "by_day_of_week", "day_of_week"} {
				if strings.Contains(text, absent) {
					t.Errorf("result included %q", absent)
				}
			}
		})
	}
}

func TestBestTravelWindowAgreesWithMonthLists(t *testing.T) {
	leading := store.AssembleRouteTravelWindows(testCodeORD, testCodeLAX, "", testStatsStart, testStatsEnd, 200, []store.TravelWindowBucketCount{
		{Grain: store.TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 54, Flights: 60},
		{Grain: store.TravelWindowGrainMonth, Bucket: 6, OnTimeCount: 200, Flights: 400},
		{Grain: store.TravelWindowGrainMonth, Bucket: 4, OnTimeCount: 20, Flights: 40},
	})
	tied := store.AssembleRouteTravelWindows(testCodeORD, testCodeLAX, "", testStatsStart, testStatsEnd, 200, []store.TravelWindowBucketCount{
		{Grain: store.TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 97, Flights: 110},
		{Grain: store.TravelWindowGrainMonth, Bucket: 2, OnTimeCount: 2550, Flights: 3000},
		{Grain: store.TravelWindowGrainMonth, Bucket: 3, OnTimeCount: 2460, Flights: 3000},
	})

	tests := []struct {
		name    string
		windows *model.RouteTravelWindows
		want    string
		best    []int
		worst   []int
	}{
		{
			name:    "sixty flight month leads",
			windows: leading,
			want:    "January is the most reliable month for ORD–LAX (90% on time), and June and April are the least reliable (50%). Medium confidence, 40–400 flights each.",
			best:    []int{1},
			worst:   []int{6, 4},
		},
		{
			name:    "wilson leader separates the lower month",
			windows: tied,
			want:    "January and February are the most reliable months for ORD–LAX (88% and 85% on time), and March is the least reliable (82%). High confidence, 110–3000 flights each.",
			best:    []int{1, 2},
			worst:   []int{3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &storetest.Stub{
				RouteTravelWindowsFn: func(context.Context, store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
					return tt.windows, nil
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
			if out.Summary != tt.want {
				t.Fatalf("summary = %q", out.Summary)
			}

			if out.Data.MonthComparison != model.ComparisonRanked {
				t.Fatalf("month_comparison = %q", out.Data.MonthComparison)
			}

			if !slices.Equal(out.Data.BestMonths, tt.best) || !slices.Equal(out.Data.WorstMonths, tt.worst) {
				t.Fatalf("best = %+v worst = %+v", out.Data.BestMonths, out.Data.WorstMonths)
			}

			if strings.Contains(out.Summary, "April (low confidence") || strings.Contains(out.Summary, "thin data") {
				t.Fatalf("summary hid a month: %s", out.Summary)
			}
		})
	}
}

func TestBestTravelWindowListsDoNotOverlap(t *testing.T) {
	windows := store.AssembleRouteTravelWindows(testCodeORD, testCodeLAX, "", testStatsStart, testStatsEnd, 6060, []store.TravelWindowBucketCount{
		{Grain: store.TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 2700, Flights: 3000},
		{Grain: store.TravelWindowGrainMonth, Bucket: 2, OnTimeCount: 2580, Flights: 3000},
		{Grain: store.TravelWindowGrainMonth, Bucket: 3, OnTimeCount: 51, Flights: 60},
	})

	stub := &storetest.Stub{
		RouteTravelWindowsFn: func(context.Context, store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
			return windows, nil
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
	want := "January is the most reliable month for ORD–LAX (90% on time), and February is the least reliable (86%). March is too close to call against January (85%). High confidence, 60–3000 flights each."

	if out.Summary != want {
		t.Fatalf("summary = %q", out.Summary)
	}

	if out.Data.MonthComparison != model.ComparisonRanked {
		t.Fatalf("month_comparison = %q", out.Data.MonthComparison)
	}

	if !slices.Equal(out.Data.BestMonths, []int{1, 3}) {
		t.Fatalf("best = %+v", out.Data.BestMonths)
	}

	if !slices.Equal(out.Data.WorstMonths, []int{2}) {
		t.Fatalf("worst = %+v", out.Data.WorstMonths)
	}

	if listsOverlap(out.Data.BestMonths, out.Data.WorstMonths) {
		t.Fatalf("best %+v overlaps worst %+v", out.Data.BestMonths, out.Data.WorstMonths)
	}

	for _, name := range []string{"January", "February", "March"} {
		if !strings.Contains(out.Summary, name) {
			t.Fatalf("summary dropped %s: %s", name, out.Summary)
		}
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
	if len(out.Data.ByMonth) != 3 {
		t.Fatalf("months = %#v", out.Data.ByMonth)
	}

	if out.Data.ByMonth[0].SampleSize != 80 || out.Data.ByMonth[0].SampleReason != model.SampleReasonSufficient || out.Data.ByMonth[0].Confidence != model.ConfidenceHigh {
		t.Fatalf("solid = %#v", out.Data.ByMonth[0])
	}

	if out.Data.ByMonth[1].SampleSize != 4 || out.Data.ByMonth[1].SampleReason != model.SampleReasonInsufficientSample || out.Data.ByMonth[1].Confidence != model.ConfidenceLow {
		t.Fatalf("thin = %#v", out.Data.ByMonth[1])
	}

	if out.Data.ByMonth[2].SampleSize != 2 || out.Data.ByMonth[2].SampleReason != model.SampleReasonInsufficientSample || out.Data.ByMonth[2].Confidence != "" {
		t.Fatalf("unknown = %#v", out.Data.ByMonth[2])
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
			wantText: testRouteNotFound,
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

func listsOverlap(best, worst []int) bool {
	seen := make(map[int]struct{}, len(best))
	for _, month := range best {
		seen[month] = struct{}{}
	}

	for _, month := range worst {
		if _, ok := seen[month]; ok {
			return true
		}
	}

	return false
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
	counts := make([]store.TravelWindowBucketCount, 0, len(months))
	for _, month := range months {
		counts = append(counts, store.TravelWindowBucketCount{
			Grain:       store.TravelWindowGrainMonth,
			Bucket:      month.Month,
			OnTimeCount: int(math.Round(month.OnTimeRate * float64(month.Flights))),
			Flights:     month.Flights,
		})
	}

	windows := store.AssembleRouteTravelWindows(testCodeORD, testCodeLAX, "", testStatsStart, testStatsEnd, 4242, counts)
	planted := make(map[int]model.TravelWindowMonthBucket, len(months))

	for _, month := range months {
		planted[month.Month] = month
	}

	restoreMonths(windows.ByMonth, planted)
	restoreMonths(windows.BestMonths, planted)
	restoreMonths(windows.WorstMonths, planted)
	windows.ByDayOfWeek = []model.TravelWindowDayBucket{{
		DayOfWeek:   1,
		OnTimeRate:  0.13,
		Flights:     80,
		Confidence:  model.ConfidenceHigh,
		Reliability: model.ReliabilityTypical,
	}}
	windows.ByHour = []model.TravelWindowHourBucket{{
		Hour:        7,
		OnTimeRate:  0.17,
		Flights:     40,
		Confidence:  model.ConfidenceHigh,
		Reliability: model.ReliabilityTypical,
	}}
	windows.Analysis.SampleSize = 4242

	return windows
}

func restoreMonths(buckets []model.TravelWindowMonthBucket, planted map[int]model.TravelWindowMonthBucket) {
	for i := range buckets {
		src, ok := planted[buckets[i].Month]
		if !ok {
			continue
		}

		buckets[i].OnTimeRate = src.OnTimeRate
		buckets[i].Confidence = src.Confidence
		buckets[i].Reliability = src.Reliability
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

		if _, ok := row["flights"]; ok {
			t.Fatalf("item %d still has flights: %#v", i, row)
		}
	}
}
