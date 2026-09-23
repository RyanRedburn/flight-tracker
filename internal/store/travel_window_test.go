package store

import (
	"testing"
	"time"
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

func TestTravelWindowMinSample(t *testing.T) {
	tests := []struct {
		name          string
		floor         int
		windowFlights int
		want          int
	}{
		{name: "hour floor beats small share", floor: TravelWindowHourFloor, windowFlights: 100, want: 30},
		{name: "day floor beats small share", floor: TravelWindowDayFloor, windowFlights: 100, want: 50},
		{name: "one percent of large window", floor: TravelWindowMonthFloor, windowFlights: 10000, want: 100},
		{name: "ceil one percent", floor: TravelWindowHourFloor, windowFlights: 3100, want: 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TravelWindowMinSample(tt.floor, tt.windowFlights); got != tt.want {
				t.Errorf("TravelWindowMinSample(%d, %d) = %d, want %d", tt.floor, tt.windowFlights, got, tt.want)
			}
		})
	}
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

	if got.Origin != testTravelOrigin || got.Dest != testTravelDest || got.Carrier != "" {
		t.Fatalf("identity = %+v", got)
	}

	if got.WindowStart != testTravelWindowStart || got.WindowEnd != testTravelWindowEnd {
		t.Errorf("window = %s..%s", got.WindowStart, got.WindowEnd)
	}

	if len(got.ByMonth) != 4 {
		t.Fatalf("by_month len = %d, want 4 (flights >= 1)", len(got.ByMonth))
	}

	if got.ByMonth[0].Month != 1 || got.ByMonth[0].OnTimeRate != 0.9 {
		t.Errorf("first month = %+v", got.ByMonth[0])
	}

	if len(got.ByHour) != 2 {
		t.Fatalf("by_hour len = %d, want 2 (hour 0 and 7; skip flights 0)", len(got.ByHour))
	}

	if got.ByHour[0].Hour != 0 {
		t.Errorf("first hour = %d, want 0", got.ByHour[0].Hour)
	}

	// window 200 → 1% = 2; month floor 50. Month 4 (49 flights) is in the series but not best/worst.
	if len(got.BestMonths) != 3 {
		t.Fatalf("best_months len = %d, want 3", len(got.BestMonths))
	}

	if got.BestMonths[0].Month != 1 || got.BestMonths[1].Month != 3 || got.BestMonths[2].Month != 2 {
		t.Errorf("best_months = %+v", got.BestMonths)
	}

	if len(got.WorstMonths) != 3 {
		t.Fatalf("worst_months len = %d, want 3", len(got.WorstMonths))
	}

	if got.WorstMonths[0].Month != 2 {
		t.Errorf("worst_months[0] = %+v, want month 2", got.WorstMonths[0])
	}

	if len(got.BestDays) != 1 || got.BestDays[0].DayOfWeek != 1 {
		t.Errorf("best_days = %+v (day 2 has 40 < floor 50)", got.BestDays)
	}

	if len(got.WorstDays) != 1 {
		t.Errorf("worst_days = %+v, want 1 eligible", got.WorstDays)
	}

	if len(got.BestHours) != 1 || got.BestHours[0].Hour != 0 {
		t.Errorf("best_hours = %+v (hour 7 has 29 < floor 30)", got.BestHours)
	}

	again := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "", testTravelWindowStart, testTravelWindowEnd, 200, counts)
	if again.ByMonth[0] != got.ByMonth[0] || again.BestHours[0] != got.BestHours[0] {
		t.Error("AssembleRouteTravelWindows is not idempotent for the same counts")
	}
}

func TestAssembleRouteTravelWindowsEligibilityShare(t *testing.T) {
	counts := []TravelWindowBucketCount{
		{Grain: TravelWindowGrainMonth, Bucket: 1, OnTimeCount: 90, Flights: 100},
		{Grain: TravelWindowGrainMonth, Bucket: 6, OnTimeCount: 40, Flights: 50},
	}

	got := AssembleRouteTravelWindows(testTravelOrigin, testTravelDest, "UA", testTravelWindowStart, testTravelWindowEnd, 10000, counts)
	if got.Carrier != "UA" {
		t.Fatalf("carrier = %q, want UA", got.Carrier)
	}

	if len(got.ByMonth) != 2 {
		t.Fatalf("by_month len = %d, want 2", len(got.ByMonth))
	}

	if len(got.BestMonths) != 1 || got.BestMonths[0].Month != 1 {
		t.Errorf("best_months = %+v, want only month 1 (50 < ceil(0.01*10000)=100)", got.BestMonths)
	}

	if len(got.WorstMonths) != 1 {
		t.Errorf("worst_months = %+v, want 1", got.WorstMonths)
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
	if len(got.BestHours) != 3 {
		t.Fatalf("best_hours len = %d, want 3", len(got.BestHours))
	}

	want := []int{6, 7, 8}
	for i, hour := range want {
		if got.BestHours[i].Hour != hour {
			t.Errorf("best_hours[%d] = %d, want %d", i, got.BestHours[i].Hour, hour)
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

	if len(got.BestMonths) != 0 || len(got.WorstMonths) != 0 {
		t.Errorf("best/worst months should be empty, got %v / %v", got.BestMonths, got.WorstMonths)
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
}
