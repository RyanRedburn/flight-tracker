package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestMinutesToHHMM(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{minutes: 0, want: "0000"},
		{minutes: 420, want: "0700"},
		{minutes: 1439, want: "2359"},
		{minutes: 1440, want: "0000"},
		{minutes: 1500, want: "0100"},
		{minutes: -5, want: "0000"},
	}

	for _, tt := range tests {
		if got := MinutesToHHMM(tt.minutes); got != tt.want {
			t.Errorf("MinutesToHHMM(%d) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}

func TestCarrierOnTimeMatchesAggregateOnTime(t *testing.T) {
	// Same classification as QueryRouteStats, including the per-carrier counts
	// folded into that query: cancelled, else diverted, else delayed when
	// arr_del15 >= 1, else on-time.
	type flight struct {
		carrier                       string
		cancelled, diverted, arrDel15 float64
	}

	flights := []flight{
		{carrier: "UA", cancelled: 0, diverted: 0, arrDel15: 0},
		{carrier: "UA", cancelled: 0, diverted: 0, arrDel15: 1},
		{carrier: "UA", cancelled: 1, diverted: 0, arrDel15: 0},
		{carrier: "AA", cancelled: 0, diverted: 0, arrDel15: 0},
		{carrier: "AA", cancelled: 0, diverted: 1, arrDel15: 0},
		{carrier: "DL", cancelled: 0, diverted: 0, arrDel15: 0.5},
	}

	type counts struct {
		flights, onTime int
	}

	var agg counts

	byCarrier := map[string]counts{}

	for _, f := range flights {
		onTime := classifyRouteStatsOnTime(f.cancelled, f.diverted, f.arrDel15)
		agg.flights++
		agg.onTime += onTime

		cur := byCarrier[f.carrier]
		cur.flights++
		cur.onTime += onTime
		byCarrier[f.carrier] = cur
	}

	if agg.onTime != 3 || agg.flights != 6 {
		t.Fatalf("aggregate on_time/flights = %d/%d, want 3/6 (not cancelled/diverted, arr_del15 < 1)", agg.onTime, agg.flights)
	}

	aggRate := rate(agg.onTime, agg.flights)

	var (
		items      []model.CarrierOnTime
		sumFlights int
	)

	for carrier, c := range byCarrier {
		item := CarrierOnTimeFromCounts(carrier, c.onTime, c.flights)
		if item.OnTimeRate != rate(c.onTime, c.flights) {
			t.Errorf("%s on_time_rate = %v, want on_time_count/flights", carrier, item.OnTimeRate)
		}

		items = append(items, item)
		sumFlights += item.Flights
	}

	if sumFlights != agg.flights {
		t.Fatalf("per-carrier flights %d do not partition aggregate %d", sumFlights, agg.flights)
	}

	if aggRate != rate(3, 6) {
		t.Fatalf("aggregate on_time_rate = %v, want 3/6", aggRate)
	}

	SortCarrierOnTime(items)

	wantOrder := []string{"DL", "AA", "UA"}
	if len(items) != len(wantOrder) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(wantOrder))
	}

	for i, carrier := range wantOrder {
		if items[i].Carrier != carrier {
			t.Errorf("items[%d].Carrier = %q, want %q", i, items[i].Carrier, carrier)
		}
	}
}

func TestCarrierComparison(t *testing.T) {
	tests := []struct {
		name  string
		items []model.CarrierOnTime
		want  string
	}{
		{name: "one carrier", items: []model.CarrierOnTime{{Carrier: "UA", OnTimeRate: 0.8, Flights: 1}}, want: ""},
		{
			name: "a low-flight carrier is compared",
			items: []model.CarrierOnTime{
				{Carrier: "UA", OnTimeRate: 0.9, Flights: model.CarrierLowSampleBelow},
				{Carrier: "AA", OnTimeRate: 0.1, Flights: 1},
			},
			want: model.ComparisonTooCloseToCall,
		},
		{
			name: "zero flights are not compared",
			items: []model.CarrierOnTime{
				{Carrier: "UA", OnTimeRate: 0.9, Flights: 1},
				{Carrier: "AA", OnTimeRate: 0.1, Flights: 0},
			},
			want: "",
		},
		{
			name: "ten point gap on uneven samples",
			items: []model.CarrierOnTime{
				{Carrier: "NK", OnTimeRate: 0.77, Flights: 120},
				{Carrier: "F9", OnTimeRate: 0.67, Flights: 180},
			},
			want: model.ComparisonTooCloseToCall,
		},
		{
			name: "wide gap",
			items: []model.CarrierOnTime{
				{Carrier: "UA", OnTimeRate: 0.9, Flights: 200},
				{Carrier: "AA", OnTimeRate: 0.5, Flights: 200},
			},
			want: model.ComparisonRanked,
		},
		{
			name: "wilson lower bound leads the large sample",
			items: []model.CarrierOnTime{
				{Carrier: "UA", OnTimeRate: 0.88, Flights: 110},
				{Carrier: "AA", OnTimeRate: 0.85, Flights: 3000},
				{Carrier: "DL", OnTimeRate: 0.82, Flights: 3000},
			},
			want: model.ComparisonRanked,
		},
		{
			name: "leader differs from another carrier",
			items: []model.CarrierOnTime{
				{Carrier: "DL", OnTimeRate: 0.82, Flights: 3000},
				{Carrier: "AA", OnTimeRate: 0.85, Flights: 3000},
				{Carrier: "UA", OnTimeRate: 0.90, Flights: 400},
			},
			want: model.ComparisonRanked,
		},
		{
			name: "counts keep a rounded gap inside the margin",
			items: []model.CarrierOnTime{
				carrierCounted("UA", 64, 100, 0.64),
				carrierCounted("AA", 51, 101, 0.50),
			},
			want: model.ComparisonTooCloseToCall,
		},
		{
			name: "the same rounded rates without counts are ranked",
			items: []model.CarrierOnTime{
				{Carrier: "UA", OnTimeRate: 0.64, Flights: 100},
				{Carrier: "AA", OnTimeRate: 0.50, Flights: 101},
			},
			want: model.ComparisonRanked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CarrierComparison(tt.items); got != tt.want {
				t.Fatalf("CarrierComparison() = %q, want %q", got, tt.want)
			}
		})
	}
}

func carrierCounted(carrier string, onTime, flights int, rounded float64) model.CarrierOnTime {
	item := CarrierOnTimeFromCounts(carrier, onTime, flights)
	item.OnTimeRate = rounded

	return item
}

func TestSortCarrierOnTime(t *testing.T) {
	items := []model.CarrierOnTime{
		{Carrier: "UA", OnTimeRate: 0.5, Flights: 2},
		{Carrier: "B6", OnTimeRate: 0.5, Flights: 2},
		{Carrier: "AA", OnTimeRate: 0.5, Flights: 10},
		{Carrier: "DL", OnTimeRate: 1, Flights: 1},
	}

	SortCarrierOnTime(items)

	want := []string{"DL", "AA", "B6", "UA"}
	for i, carrier := range want {
		if items[i].Carrier != carrier {
			t.Errorf("items[%d].Carrier = %q, want %q", i, items[i].Carrier, carrier)
		}
	}
}

func TestParseOutlookDay(t *testing.T) {
	day, err := ParseOutlookDay("2026-10-06", 0)
	if err != nil || day != 2 {
		t.Fatalf("date day = %d, %v", day, err)
	}

	day, err = ParseOutlookDay("", 7)
	if err != nil || day != 7 {
		t.Fatalf("weekday = %d, %v", day, err)
	}

	if _, err := ParseOutlookDay("", 0); !errors.Is(err, ErrOutlookSchedule) {
		t.Fatalf("missing schedule error = %v", err)
	}

	if _, err := ParseOutlookDay("2026-10-06", 2); !errors.Is(err, ErrOutlookSchedule) {
		t.Fatalf("both set error = %v", err)
	}

	if _, err := ParseOutlookDay("2026-13-01", 0); !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("bad date error = %v", err)
	}

	if _, err := ParseOutlookDay("", 8); !errors.Is(err, ErrInvalidWeekday) {
		t.Fatalf("bad weekday error = %v", err)
	}
}

func classifyRouteStatsOnTime(cancelled, diverted, arrDel15 float64) int {
	switch {
	case cancelled >= 1:
		return 0
	case diverted >= 1:
		return 0
	case arrDel15 >= 1:
		return 0
	default:
		return 1
	}
}

func TestTrailingOutlookWindow(t *testing.T) {
	start, end, ok := TrailingOutlookWindow("2026-04-01")
	if !ok || end != "2026-04-01" {
		t.Fatalf("window = %s %s ok=%v", start, end, ok)
	}

	startTime, err := time.Parse("2006-01-02", start)
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}

	endTime, err := time.Parse("2006-01-02", end)
	if err != nil {
		t.Fatalf("parse end: %v", err)
	}

	if !startTime.Equal(endTime.AddDate(0, 0, -OutlookLookbackDays)) {
		t.Fatalf("start = %s", start)
	}

	if ExceedsMaxStatsSpan(startTime, endTime) {
		t.Fatal("default window exceeds max stats span")
	}

	if _, _, ok := TrailingOutlookWindow("not-a-date"); ok {
		t.Fatal("expected invalid date")
	}
}

func TestApplyOutlookLookback(t *testing.T) {
	if err := ApplyOutlookLookback(nil, false, func(int) error { return nil }); err != nil {
		t.Fatalf("nil outlook: %v", err)
	}

	t.Run("thin default window widens", func(t *testing.T) {
		out := &model.RouteOutlook{}

		var calls []int

		err := ApplyOutlookLookback(out, false, func(days int) error {
			calls = append(calls, days)
			if days == OutlookLookbackDays {
				out.Analysis.SampleSize = model.OutlookLowSampleBelow - 1

				return nil
			}

			out.Analysis.SampleSize = 40

			return nil
		})
		if err != nil {
			t.Fatalf("ApplyOutlookLookback() error = %v", err)
		}

		if len(calls) != 2 || calls[0] != OutlookLookbackDays || calls[1] != OutlookFallbackLookbackDays {
			t.Fatalf("calls = %v", calls)
		}

		if out.Analysis.LookbackDays != OutlookFallbackLookbackDays {
			t.Fatalf("lookback_days = %d", out.Analysis.LookbackDays)
		}

		if out.Analysis.SampleReason != model.SampleReasonSufficient || out.Analysis.Confidence != model.ConfidenceMedium {
			t.Fatalf("sample = %+v", out.Analysis)
		}
	})

	t.Run("caller window does not widen", func(t *testing.T) {
		out := &model.RouteOutlook{}

		var calls []int

		err := ApplyOutlookLookback(out, true, func(days int) error {
			calls = append(calls, days)
			out.Analysis.SampleSize = 1

			return nil
		})
		if err != nil {
			t.Fatalf("ApplyOutlookLookback() error = %v", err)
		}

		if len(calls) != 1 || calls[0] != OutlookLookbackDays {
			t.Fatalf("calls = %v", calls)
		}

		if out.Analysis.LookbackDays != OutlookLookbackDays || out.Analysis.Confidence != model.ConfidenceLow {
			t.Fatalf("sample = %+v", out.Analysis)
		}
	})

	t.Run("enough flights stay on the default window", func(t *testing.T) {
		out := &model.RouteOutlook{}

		var calls []int

		err := ApplyOutlookLookback(out, false, func(days int) error {
			calls = append(calls, days)
			out.Analysis.SampleSize = model.OutlookLowSampleBelow

			return nil
		})
		if err != nil {
			t.Fatalf("ApplyOutlookLookback() error = %v", err)
		}

		if len(calls) != 1 || out.Analysis.LookbackDays != OutlookLookbackDays || out.Analysis.Confidence != model.ConfidenceMedium {
			t.Fatalf("calls = %v sample = %+v", calls, out.Analysis)
		}
	})
}

func TestDepTimeWindowAndStatsSpan(t *testing.T) {
	if err := DepTimeWindowError(0); err == nil {
		t.Fatal("expected window error")
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	if !ExceedsMaxStatsSpan(start, end) {
		t.Fatal("expected span to exceed the maximum")
	}

	if ExceedsMaxStatsSpan(time.Time{}, end) {
		t.Fatal("zero start is not a span error")
	}
}
