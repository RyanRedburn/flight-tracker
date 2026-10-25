package store

import (
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	testTravelOrigin      = "ORD"
	testTravelDest        = "LAX"
	testTravelWindowStart = "2024-04-30"
	testTravelWindowEnd   = "2026-04-30"
)

func TestTravelWindowBounds(t *testing.T) {
	maxDate := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

	t.Run("caps at two years", func(t *testing.T) {
		minDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		start, end := TravelWindowBounds(minDate, maxDate)

		if got := start.Format("2006-01-02"); got != testTravelWindowStart {
			t.Errorf("start = %s, want %s", got, testTravelWindowStart)
		}

		if got := end.Format("2006-01-02"); got != testTravelWindowEnd {
			t.Errorf("end = %s, want %s", got, testTravelWindowEnd)
		}
	})

	t.Run("uses shorter history", func(t *testing.T) {
		minDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
		start, end := TravelWindowBounds(minDate, maxDate)

		if got := start.Format("2006-01-02"); got != "2025-01-15" {
			t.Errorf("start = %s, want 2025-01-15", got)
		}

		if !end.Equal(maxDate) {
			t.Errorf("end = %s, want %s", end.Format("2006-01-02"), maxDate.Format("2006-01-02"))
		}
	})
}

func TestAssembleRouteTravelWindows(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 90, Flights: 100},
		{Grain: TravelWindowGrainMonth, Bucket: 2, OnTimeCount: 30, Flights: 50},
		{Grain: TravelWindowGrainMonth, Bucket: 3, OnTimeCount: 80, Flights: 100},
		{Grain: TravelWindowGrainMonth, Bucket: 4, OnTimeCount: 20, Flights: 49},
		{Grain: TravelWindowGrainDayOfWeek, Bucket: 1, OnTimeCount: 70, Flights: 80},
		{Grain: TravelWindowGrainDayOfWeek, Bucket: 2, OnTimeCount: 10, Flights: 40},
		{Grain: TravelWindowGrainHour, Bucket: 0, OnTimeCount: 30, Flights: 40},
		{Grain: TravelWindowGrainHour, Bucket: 7, OnTimeCount: 20, Flights: 29},
		{Grain: TravelWindowGrainHour, Bucket: 8, OnTimeCount: 1, Flights: 0},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 200, counts)

	if got.Filters.Origin != testTravelOrigin || got.Filters.Dest != testTravelDest || got.Filters.Carrier != "" {
		t.Fatalf("identity = %+v", got)
	}

	if got.Analysis.Start != testTravelWindowStart || got.Analysis.End != testTravelWindowEnd || got.Analysis.SampleSize != 200 {
		t.Errorf("window = %s..%s sample %d", got.Analysis.Start, got.Analysis.End, got.Analysis.SampleSize)
	}

	if len(got.ByMonth) != 4 {
		t.Fatalf("by_month len = %d, want 4 (flights >= 1)", len(got.ByMonth))
	}

	if got.ByMonth[0].Month != 1 || got.ByMonth[0].OnTimeRate != 0.9 || got.ByMonth[0].Confidence != model.ConfidenceHigh || got.ByMonth[0].Reliability != model.ReliabilityReliable {
		t.Errorf("first month = %+v", got.ByMonth[0])
	}

	if got.ByMonth[1].Confidence != model.ConfidenceHigh || got.ByMonth[1].Reliability != model.ReliabilityUnreliable {
		t.Errorf("month 2 = %+v, want high/unreliable", got.ByMonth[1])
	}

	if got.ByMonth[3].Confidence != model.ConfidenceMedium || got.ByMonth[3].Reliability != model.ReliabilityUnreliable {
		t.Errorf("month 4 = %+v, want medium/unreliable", got.ByMonth[3])
	}

	if got.BestMonths[0].Confidence != model.ConfidenceHigh || got.BestMonths[0].Reliability != model.ReliabilityReliable {
		t.Errorf("best month decision = %+v", got.BestMonths[0])
	}

	if len(got.ByHour) != 2 {
		t.Fatalf("by_hour len = %d, want 2 (hour 0 and 7; skip flights 0)", len(got.ByHour))
	}

	if got.ByHour[0].Hour != 0 || got.ByHour[0].Confidence != model.ConfidenceMedium || got.ByHour[0].Reliability != model.ReliabilityTypical {
		t.Errorf("first hour = %+v, want hour 0 medium/typical", got.ByHour[0])
	}

	if got.ByHour[1].Confidence != model.ConfidenceLow || got.ByHour[1].Reliability != model.ReliabilityUnknown {
		t.Errorf("hour 7 = %+v, want low/unknown", got.ByHour[1])
	}

	// Month 1 leads and differs from the others. Month 4 is tied with month 2 at the bottom.
	if got.MonthComparison != model.ComparisonRanked {
		t.Fatalf("month_comparison = %q, want ranked", got.MonthComparison)
	}

	if len(got.BestMonths) != 1 || got.BestMonths[0].Month != 1 {
		t.Fatalf("best_months = %+v, want month 1", got.BestMonths)
	}

	if len(got.WorstMonths) != 2 || got.WorstMonths[0].Month != 4 || got.WorstMonths[1].Month != 2 {
		t.Fatalf("worst_months = %+v, want months 4 then 2", got.WorstMonths)
	}

	if got.DayComparison != model.ComparisonRanked || len(got.BestDays) != 1 || got.BestDays[0].DayOfWeek != 1 || len(got.WorstDays) != 1 || got.WorstDays[0].DayOfWeek != 2 {
		t.Errorf("days = best %+v worst %+v comparison %q", got.BestDays, got.WorstDays, got.DayComparison)
	}

	if got.HourComparison != model.ComparisonTooCloseToCall || len(got.BestHours) != 0 || len(got.WorstHours) != 0 {
		t.Errorf("hours = best %+v worst %+v comparison %q", got.BestHours, got.WorstHours, got.HourComparison)
	}

	again := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 200, counts)
	if again.ByHour[0].Hour != got.ByHour[0].Hour || again.ByHour[0].OnTimeRate != got.ByHour[0].OnTimeRate || again.ByMonth[0].Month != got.ByMonth[0].Month || again.ByMonth[0].OnTimeRate != got.ByMonth[0].OnTimeRate || again.ByMonth[0].Flights != got.ByMonth[0].Flights || again.ByMonth[0].Confidence != got.ByMonth[0].Confidence || again.ByMonth[0].Reliability != got.ByMonth[0].Reliability {
		t.Error("AssembleRouteTravelWindows is not idempotent for the same counts")
	}

	if again.ByMonth[0].OnTimeCount == nil || got.ByMonth[0].OnTimeCount == nil || *again.ByMonth[0].OnTimeCount != *got.ByMonth[0].OnTimeCount {
		t.Error("on-time count was not preserved")
	}
}

func TestMonthConfidenceBands(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 20, Flights: 29},
		{Grain: TravelWindowGrainMonth, Bucket: 2, OnTimeCount: 24, Flights: 30},
		{Grain: TravelWindowGrainMonth, Bucket: 3, OnTimeCount: 20, Flights: 49},
		{Grain: TravelWindowGrainMonth, Bucket: 4, OnTimeCount: 40, Flights: 50},
		{Grain: TravelWindowGrainDayOfWeek, Bucket: 1, OnTimeCount: 40, Flights: 50},
		{Grain: TravelWindowGrainHour, Bucket: 8, OnTimeCount: 24, Flights: 30},
		{Grain: TravelWindowGrainHour, Bucket: 9, OnTimeCount: 20, Flights: 29},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 200, counts)

	if got.ByMonth[0].Confidence != model.ConfidenceLow || got.ByMonth[0].Reliability != model.ReliabilityUnknown {
		t.Errorf("29 flights = %+v, want low/unknown", got.ByMonth[0])
	}

	if got.ByMonth[1].Confidence != model.ConfidenceMedium || got.ByMonth[1].Reliability != model.ReliabilityReliable {
		t.Errorf("30 flights = %+v, want medium/reliable", got.ByMonth[1])
	}

	if got.ByMonth[2].Confidence != model.ConfidenceMedium || got.ByMonth[2].Reliability != model.ReliabilityUnreliable {
		t.Errorf("49 flights = %+v, want medium/unreliable", got.ByMonth[2])
	}

	if got.ByMonth[3].Confidence != model.ConfidenceHigh || got.ByMonth[3].Reliability != model.ReliabilityReliable {
		t.Errorf("50 flights = %+v, want high/reliable", got.ByMonth[3])
	}

	if got.ByDayOfWeek[0].Confidence != model.ConfidenceHigh || got.ByDayOfWeek[0].Reliability != model.ReliabilityReliable {
		t.Errorf("weekday = %+v, want high/reliable", got.ByDayOfWeek[0])
	}

	if got.ByHour[0].Confidence != model.ConfidenceMedium || got.ByHour[1].Confidence != model.ConfidenceLow {
		t.Errorf("hours = %+v, want medium then low", got.ByHour)
	}
}

func TestAssembleRouteTravelWindowsEligibilityShare(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 90, Flights: 100},
		{Grain: TravelWindowGrainMonth, Bucket: 6, OnTimeCount: 40, Flights: 50},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "UA", testTravelWindowStart, testTravelWindowEnd, 10000, counts)
	if got.Filters.Carrier != "UA" {
		t.Fatalf("carrier = %q, want UA", got.Filters.Carrier)
	}

	if len(got.ByMonth) != 2 {
		t.Fatalf("by_month len = %d, want 2", len(got.ByMonth))
	}

	if got.MonthComparison != model.ComparisonTooCloseToCall || len(got.BestMonths) != 0 || len(got.WorstMonths) != 0 {
		t.Errorf("best=%+v worst=%+v comparison=%q, want too close", got.BestMonths, got.WorstMonths, got.MonthComparison)
	}
}

func TestAssembleRouteTravelWindowsLowSampleMonthCanLead(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 54, Flights: 60},
		{Grain: TravelWindowGrainMonth, Bucket: 6, OnTimeCount: 200, Flights: 400},
		{Grain: TravelWindowGrainMonth, Bucket: 4, OnTimeCount: 20, Flights: 40},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 200, counts)
	if got.MonthComparison != model.ComparisonRanked {
		t.Fatalf("month_comparison = %q, want ranked", got.MonthComparison)
	}

	if len(got.BestMonths) != 1 || got.BestMonths[0].Month != 1 || got.BestMonths[0].Flights != 60 {
		t.Fatalf("best_months = %+v, want the 60-flight month", got.BestMonths)
	}

	if got.BestMonths[0].Confidence != model.ConfidenceHigh || got.BestMonths[0].Reliability != model.ReliabilityReliable {
		t.Fatalf("leading month = %+v, want high/reliable", got.BestMonths[0])
	}

	if len(got.WorstMonths) != 2 || got.WorstMonths[0].Month != 6 || got.WorstMonths[1].Month != 4 {
		t.Fatalf("worst_months = %+v, want months 6 and 4", got.WorstMonths)
	}
}

func TestRankTravelMonthsWilsonLeader(t *testing.T) {
	best, worst, comparison := rankTravelBuckets([]model.TravelWindowMonthBucket{
		{Month: 1, OnTimeRate: 0.88, Flights: 110},
		{Month: 2, OnTimeRate: 0.85, Flights: 3000},
		{Month: 3, OnTimeRate: 0.82, Flights: 3000},
	}, monthBucketKey, monthBucketRate, monthBucketCount, monthBucketFlights)

	if comparison != model.ComparisonRanked {
		t.Fatalf("comparison = %q, want ranked", comparison)
	}

	if len(best) != 2 || best[0].Month != 2 || best[1].Month != 1 {
		t.Fatalf("best = %+v, want the 3000-flight month then the thin 88%%", best)
	}

	if len(worst) != 1 || worst[0].Month != 3 {
		t.Fatalf("worst = %+v, want the 82%% month", worst)
	}
}

func TestAssembleRouteTravelWindowsMonthTooClose(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 80, Flights: 100},
		{Grain: TravelWindowGrainMonth, Bucket: 6, OnTimeCount: 78, Flights: 100},
		{Grain: TravelWindowGrainDayOfWeek, Bucket: 1, OnTimeCount: 90, Flights: 100},
		{Grain: TravelWindowGrainDayOfWeek, Bucket: 2, OnTimeCount: 40, Flights: 100},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 400, counts)
	if got.MonthComparison != model.ComparisonTooCloseToCall {
		t.Fatalf("month_comparison = %q", got.MonthComparison)
	}

	if len(got.BestMonths) != 0 || len(got.WorstMonths) != 0 {
		t.Fatalf("best/worst months = %v / %v, want empty", got.BestMonths, got.WorstMonths)
	}

	if len(got.ByMonth) != 2 {
		t.Fatalf("by_month len = %d, want 2", len(got.ByMonth))
	}

	if got.DayComparison != model.ComparisonRanked || len(got.BestDays) != 1 || got.BestDays[0].DayOfWeek != 1 || len(got.WorstDays) != 1 || got.WorstDays[0].DayOfWeek != 2 {
		t.Fatalf("days = best %+v worst %+v comparison %q", got.BestDays, got.WorstDays, got.DayComparison)
	}
}

func TestAssembleRouteTravelWindowsTieBreak(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainHour, Bucket: 9, OnTimeCount: 50, Flights: 100},
		{Grain: TravelWindowGrainHour, Bucket: 8, OnTimeCount: 50, Flights: 100},
		{Grain: TravelWindowGrainHour, Bucket: 7, OnTimeCount: 100, Flights: 200},
		{Grain: TravelWindowGrainHour, Bucket: 6, OnTimeCount: 80, Flights: 100},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", "2025-01-01", "2026-01-01", 500, counts)
	if got.HourComparison != model.ComparisonRanked || len(got.BestHours) != 1 || got.BestHours[0].Hour != 6 {
		t.Fatalf("best_hours = %+v comparison %q", got.BestHours, got.HourComparison)
	}

	want := []int{7, 8, 9}
	if len(got.WorstHours) != len(want) {
		t.Fatalf("worst_hours = %+v", got.WorstHours)
	}

	for i, hour := range want {
		if got.WorstHours[i].Hour != hour {
			t.Errorf("worst_hours[%d] = %d, want %d", i, got.WorstHours[i].Hour, hour)
		}
	}
}

func TestAssembleRouteTravelWindowsEmptyEligible(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 1, Flights: 1},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", "2026-01-01", "2026-01-02", 1, counts)
	if len(got.ByMonth) != 1 {
		t.Fatalf("by_month len = %d, want 1", len(got.ByMonth))
	}

	if len(got.BestMonths) != 0 || len(got.WorstMonths) != 0 || got.MonthComparison != model.ComparisonSingle {
		t.Errorf("best/worst months = %v / %v comparison %q", got.BestMonths, got.WorstMonths, got.MonthComparison)
	}

	if got.ByMonth[0].Confidence != model.ConfidenceLow || got.ByMonth[0].Reliability != model.ReliabilityUnknown {
		t.Errorf("thin month = %+v, want low/unknown", got.ByMonth[0])
	}

	if got.BestDays == nil || got.BestHours == nil {
		t.Fatal("best_days and best_hours must be empty slices, not nil")
	}
}

func TestAssembleRouteTravelWindowsRoundsLikeRouteStats(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 1, Flights: 3},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", "2026-01-01", "2026-01-31", 200, counts)
	if got.ByMonth[0].OnTimeRate != 0.33 {
		t.Errorf("on_time_rate = %v, want 0.33", got.ByMonth[0].OnTimeRate)
	}

	if got.ByMonth[0].Confidence != model.ConfidenceLow || got.ByMonth[0].Reliability != model.ReliabilityUnknown {
		t.Errorf("rounded thin month = %+v, want low/unknown", got.ByMonth[0])
	}
}

func TestAssembleRouteTravelWindowsBestAndWorstDoNotOverlap(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 2700, Flights: 3000},
		{Grain: TravelWindowGrainMonth, Bucket: 2, OnTimeCount: 2580, Flights: 3000},
		{Grain: TravelWindowGrainMonth, Bucket: 3, OnTimeCount: 51, Flights: 60},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 6060, counts)
	if got.MonthComparison != model.ComparisonRanked {
		t.Fatalf("month_comparison = %q", got.MonthComparison)
	}

	if len(got.BestMonths) != 2 || got.BestMonths[0].Month != 1 || got.BestMonths[1].Month != 3 {
		t.Fatalf("best_months = %+v, want January then March", got.BestMonths)
	}

	if len(got.WorstMonths) != 1 || got.WorstMonths[0].Month != 2 {
		t.Fatalf("worst_months = %+v, want February", got.WorstMonths)
	}

	seen := map[int]int{}
	for _, month := range append(append([]model.TravelWindowMonthBucket{}, got.BestMonths...), got.WorstMonths...) {
		seen[month.Month]++
		if seen[month.Month] > 1 {
			t.Fatalf("month %d is in both lists", month.Month)
		}
	}
}

func TestAssembleRouteTravelWindowsCapsTiedBuckets(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainHour, Bucket: 1, OnTimeCount: 80, Flights: 100},
		{Grain: TravelWindowGrainHour, Bucket: 2, OnTimeCount: 80, Flights: 100},
		{Grain: TravelWindowGrainHour, Bucket: 3, OnTimeCount: 80, Flights: 100},
		{Grain: TravelWindowGrainHour, Bucket: 4, OnTimeCount: 80, Flights: 100},
		{Grain: TravelWindowGrainHour, Bucket: 5, OnTimeCount: 20, Flights: 100},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", "2025-01-01", "2026-01-01", 500, counts)
	if got.HourComparison != model.ComparisonRanked || len(got.BestHours) != TravelWindowBestWorstK {
		t.Fatalf("best_hours = %+v comparison %q", got.BestHours, got.HourComparison)
	}

	for i, hour := range []int{1, 2, 3} {
		if got.BestHours[i].Hour != hour {
			t.Errorf("best_hours[%d] = %d, want %d", i, got.BestHours[i].Hour, hour)
		}
	}

	if len(got.WorstHours) != 1 || got.WorstHours[0].Hour != 5 {
		t.Fatalf("worst_hours = %+v", got.WorstHours)
	}

	for _, hour := range got.BestHours {
		if hour.Hour == 4 {
			t.Fatalf("hour 4 was capped out of best and must stay out of worst: %+v", got.WorstHours)
		}
	}
}

func TestAssembleRouteTravelWindowsWilsonHourLeader(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainHour, Bucket: 5, OnTimeCount: 2, Flights: 2},
		{Grain: TravelWindowGrainHour, Bucket: 7, OnTimeCount: 240, Flights: 300},
		{Grain: TravelWindowGrainHour, Bucket: 18, OnTimeCount: 180, Flights: 300},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", "2025-01-01", "2026-01-01", 602, counts)
	if got.HourComparison != model.ComparisonRanked {
		t.Fatalf("hour_comparison = %q", got.HourComparison)
	}

	// Hour 05 is 2/2. Its Wilson lower bound loses to hour 07, and the two-proportion test does not separate them.
	if len(got.BestHours) != 2 || got.BestHours[0].Hour != 7 || got.BestHours[1].Hour != 5 {
		t.Fatalf("best_hours = %+v, want 07 then 05", got.BestHours)
	}

	if len(got.WorstHours) != 1 || got.WorstHours[0].Hour != 18 {
		t.Fatalf("worst_hours = %+v, want 18", got.WorstHours)
	}
}

func TestAssembleRouteTravelWindowsZeroOnTimeFlight(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainHour, Bucket: 5, OnTimeCount: 0, Flights: 1},
		{Grain: TravelWindowGrainHour, Bucket: 7, OnTimeCount: 240, Flights: 300},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", "2025-01-01", "2026-01-01", 301, counts)
	if len(got.ByHour) != 2 || got.ByHour[0].Hour != 5 || got.ByHour[0].OnTimeRate != 0 || got.ByHour[0].Flights != 1 {
		t.Fatalf("by_hour = %+v, want the 0%% flight included", got.ByHour)
	}

	if got.ByHour[0].Confidence != model.ConfidenceLow || got.ByHour[0].Reliability != model.ReliabilityUnknown {
		t.Fatalf("zero-flight hour = %+v, want low/unknown", got.ByHour[0])
	}

	if got.HourComparison != model.ComparisonRanked || len(got.BestHours) != 1 || got.BestHours[0].Hour != 7 || len(got.WorstHours) != 1 || got.WorstHours[0].Hour != 5 {
		t.Fatalf("hours best %+v worst %+v comparison %q", got.BestHours, got.WorstHours, got.HourComparison)
	}
}
