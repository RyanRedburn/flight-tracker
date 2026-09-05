package store

import (
	"strconv"
	"testing"
)

const (
	testFlightDate20260401 = "2026-04-01"
	testEndDate20260430    = "2026-04-30"
	testAirportORD         = "ORD"
	testAirportLAX         = "LAX"
	testFloatNo            = "0.00"
	testFloatYes           = "1.00"
)

func TestCircularMinuteDistance(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{0, 0, 0},
		{70, 100, 30},
		{1430, 10, 20}, // 23:50 vs 00:10
		{10, 1430, 20},
		{0, 720, 720},
	}

	for _, tt := range tests {
		if got := CircularMinuteDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("CircularMinuteDistance(%d,%d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestHHMMToMinutes(t *testing.T) {
	tests := []struct {
		raw    string
		want   int
		wantOK bool
	}{
		{"0700", 420, true},
		{"700", 420, true},
		{"2350", 1430, true},
		{"2400", 0, false},
		{"ab", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := HHMMToMinutes(tt.raw)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("HHMMToMinutes(%q) = (%d,%v), want (%d,%v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestAggregateRouteStats(t *testing.T) {
	rows := []FlightPerf{
		{FlightDate: testFlightDate20260401, DayOfWeek: "3", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", FlightNumber: "100",
			CRSDepTime: "0700", ArrDelayMinutes: testFloatNo, DepDelayMinutes: testFloatNo, ArrDel15: testFloatNo, DepDel15: testFloatNo, Cancelled: testFloatNo, Diverted: testFloatNo},
		{FlightDate: "2026-04-02", DayOfWeek: "4", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", FlightNumber: "100",
			CRSDepTime: "0700", ArrDelayMinutes: "30.00", DepDelayMinutes: "20.00", ArrDel15: testFloatYes, DepDel15: testFloatYes, Cancelled: testFloatNo, Diverted: testFloatNo,
			CarrierDelay: "10.00", WeatherDelay: "5.00", NASDelay: "15.00", SecurityDelay: testFloatNo, LateAircraftDelay: testFloatNo},
		{FlightDate: "2026-04-03", DayOfWeek: "5", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", FlightNumber: "100",
			CRSDepTime: "0700", Cancelled: testFloatYes, CancellationCode: "B", Diverted: testFloatNo},
		{FlightDate: "2026-04-04", DayOfWeek: "6", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", FlightNumber: "100",
			CRSDepTime: "0700", Cancelled: testFloatNo, Diverted: testFloatYes, DivAirports: [5]string{"MDW", "", "", "", ""}},
		{FlightDate: "2026-04-05", DayOfWeek: "7", Origin: testAirportORD, Dest: "SFO", Carrier: "UA", FlightNumber: "200",
			CRSDepTime: "0900", ArrDelayMinutes: testFloatNo, ArrDel15: testFloatNo, Cancelled: testFloatNo, Diverted: testFloatNo},
	}

	stats := AggregateRouteStats(RouteStatsFilter{
		Origin: testAirportORD, Dest: testAirportLAX, StartDate: testFlightDate20260401, EndDate: testEndDate20260430,
	}, rows)

	if stats.Flights != 4 {
		t.Fatalf("flights = %d, want 4", stats.Flights)
	}

	if stats.OnTime != 1 || stats.Delayed != 1 || stats.Cancelled != 1 || stats.Diverted != 1 {
		t.Fatalf("counts = on_time=%d delayed=%d cancelled=%d diverted=%d", stats.OnTime, stats.Delayed, stats.Cancelled, stats.Diverted)
	}

	if stats.MedianArrivalDelayMinutes != 15 {
		t.Errorf("median arrival = %v, want 15", stats.MedianArrivalDelayMinutes)
	}

	if len(stats.DiversionAirports) != 1 || stats.DiversionAirports[0].Airport != "MDW" {
		t.Errorf("diversion_airports = %+v", stats.DiversionAirports)
	}

	if len(stats.CancellationCodes) != 1 || stats.CancellationCodes[0].Code != "B" {
		t.Errorf("cancellation_codes = %+v", stats.CancellationCodes)
	}

	if stats.DelayCausesAvgMinutes.NAS != 15 {
		t.Errorf("NAS cause avg = %v, want 15", stats.DelayCausesAvgMinutes.NAS)
	}

	filtered := AggregateRouteStats(RouteStatsFilter{
		Origin: testAirportORD, Dest: testAirportLAX, StartDate: testFlightDate20260401, EndDate: testEndDate20260430,
		DaysOfWeek: []int{3, 4},
	}, rows)
	if filtered.Flights != 2 {
		t.Fatalf("dow filtered flights = %d, want 2", filtered.Flights)
	}
}

func TestAggregateRouteOutlook(t *testing.T) {
	rows := make([]FlightPerf, 0, 12)

	for i := range 12 {
		day := i + 1

		date := "2025-06-" + strconv.Itoa(day)
		if day < 10 {
			date = "2025-06-0" + strconv.Itoa(day)
		}

		arrDel15 := testFloatNo
		arrDelay := testFloatNo

		if i%4 == 0 {
			arrDel15 = testFloatYes
			arrDelay = "40.00"
		}

		rows = append(rows, FlightPerf{
			FlightDate: date, DayOfWeek: "2", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA",
			CRSDepTime: "0700", ArrDelayMinutes: arrDelay, DepDelayMinutes: "5.00",
			ArrDel15: arrDel15, Cancelled: testFloatNo, Diverted: testFloatNo,
		})
	}

	rows = append(rows, FlightPerf{
		FlightDate: "2025-06-01", DayOfWeek: "2", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA",
		CRSDepTime: "2350", ArrDelayMinutes: testFloatNo, ArrDel15: testFloatNo, Cancelled: testFloatNo, Diverted: testFloatNo,
	})

	out := AggregateRouteOutlook(RouteOutlookFilter{
		Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", DayOfWeek: 2,
		DepTime: "0700", DepTimeWindowMinutes: 30,
	}, rows)

	if out.AnalysisEnd != "2025-06-12" {
		t.Errorf("analysis_end = %q, want 2025-06-12", out.AnalysisEnd)
	}

	if out.AnalysisStart != "2024-06-12" {
		t.Errorf("analysis_start = %q, want 2024-06-12", out.AnalysisStart)
	}

	if out.SampleSize != 12 {
		t.Fatalf("sample_size = %d, want 12", out.SampleSize)
	}

	if out.InsufficientSample {
		t.Fatal("expected sufficient sample")
	}

	if out.DelayProbability != 0.25 {
		t.Errorf("delay_probability = %v, want 0.25", out.DelayProbability)
	}

	midnight := AggregateRouteOutlook(RouteOutlookFilter{
		Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", DayOfWeek: 2,
		DepTime: "2350", DepTimeWindowMinutes: 30,
	}, rows)
	if midnight.SampleSize != 1 {
		t.Fatalf("midnight sample_size = %d, want 1", midnight.SampleSize)
	}

	if !midnight.InsufficientSample {
		t.Fatal("expected insufficient_sample for size 1")
	}
}

func TestAggregateRouteStatsRounding(t *testing.T) {
	rows := []FlightPerf{
		roundingPerf("2026-04-01", "3", testFloatNo, "10.40", "1.40", testFloatNo),
		roundingPerf("2026-04-02", "4", testFloatNo, "10.40", "1.40", testFloatNo),
		{
			FlightDate: "2026-04-03", DayOfWeek: "5", Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", FlightNumber: "100",
			CRSDepTime: "0700", ArrDelayMinutes: "20.40", DepDelayMinutes: "15.40", ArrDel15: testFloatYes, DepDel15: testFloatYes,
			Cancelled: testFloatNo, Diverted: testFloatNo, NASDelay: "10.40",
		},
	}

	stats := AggregateRouteStats(RouteStatsFilter{
		Origin: testAirportORD, Dest: testAirportLAX, StartDate: testFlightDate20260401, EndDate: testEndDate20260430,
	}, rows)

	if stats.OnTimeRate != 0.67 || stats.DelayRate != 0.33 {
		t.Errorf("rates = on_time=%v delay=%v, want 0.67 and 0.33", stats.OnTimeRate, stats.DelayRate)
	}

	if stats.AvgArrivalDelayMinutes != 14 {
		t.Errorf("avg_arrival_delay_minutes = %v, want 14", stats.AvgArrivalDelayMinutes)
	}

	if stats.MedianArrivalDelayMinutes != 10 {
		t.Errorf("median_arrival_delay_minutes = %v, want 10", stats.MedianArrivalDelayMinutes)
	}

	if stats.AvgArrivalDelayWhenDelayed != 20 {
		t.Errorf("avg_arrival_delay_when_delayed = %v, want 20", stats.AvgArrivalDelayWhenDelayed)
	}

	if stats.AvgDepartureDelayMinutes != 6 {
		t.Errorf("avg_departure_delay_minutes = %v, want 6", stats.AvgDepartureDelayMinutes)
	}

	if stats.AvgDepartureDelayWhenDelayed != 15 {
		t.Errorf("avg_departure_delay_when_delayed = %v, want 15", stats.AvgDepartureDelayWhenDelayed)
	}

	if stats.DelayCausesAvgMinutes.NAS != 10 {
		t.Errorf("NAS cause avg = %v, want 10", stats.DelayCausesAvgMinutes.NAS)
	}
}

func TestAggregateRouteOutlookRounding(t *testing.T) {
	rows := []FlightPerf{
		roundingPerf("2025-06-01", "2", testFloatNo, "10.40", "1.40", testFloatNo),
		roundingPerf("2025-06-08", "2", testFloatNo, "10.40", "1.40", testFloatNo),
		roundingPerf("2025-06-15", "2", testFloatYes, "20.40", "15.40", testFloatNo),
	}

	out := AggregateRouteOutlook(RouteOutlookFilter{
		Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", DayOfWeek: 2,
		DepTime: "0700", DepTimeWindowMinutes: 30,
	}, rows)

	if out.OnTimeProbability != 0.67 || out.DelayProbability != 0.33 {
		t.Errorf("probabilities = on_time=%v delay=%v, want 0.67 and 0.33", out.OnTimeProbability, out.DelayProbability)
	}

	if out.LikelyArrivalDelayMinutes != 14 {
		t.Errorf("likely_arrival_delay_minutes = %v, want 14", out.LikelyArrivalDelayMinutes)
	}

	if out.MedianArrivalDelayMinutes != 10 {
		t.Errorf("median_arrival_delay_minutes = %v, want 10", out.MedianArrivalDelayMinutes)
	}

	if out.LikelyArrivalDelayWhenDelayed != 20 {
		t.Errorf("likely_arrival_delay_when_delayed = %v, want 20", out.LikelyArrivalDelayWhenDelayed)
	}

	if out.LikelyDepartureDelayMinutes != 6 {
		t.Errorf("likely_departure_delay_minutes = %v, want 6", out.LikelyDepartureDelayMinutes)
	}
}

func roundingPerf(date, dayOfWeek, arrDel15, arrDelay, depDelay, cancelled string) FlightPerf {
	return FlightPerf{
		FlightDate: date, DayOfWeek: dayOfWeek, Origin: testAirportORD, Dest: testAirportLAX, Carrier: "UA", FlightNumber: "100",
		CRSDepTime: "0700", ArrDelayMinutes: arrDelay, DepDelayMinutes: depDelay, ArrDel15: arrDel15,
		Cancelled: cancelled, Diverted: testFloatNo,
	}
}
