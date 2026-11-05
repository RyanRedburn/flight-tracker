package store

import (
	"errors"
	"math"
	"sort"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/domain"
)

func TestRouteFilterValidate(t *testing.T) {
	outlook := RouteOutlookFilter{DayOfWeek: 2, DepTime: "0700", DepTimeWindowMinutes: 30}
	if err := outlook.Validate(); err != nil {
		t.Fatalf("outlook Validate: %v", err)
	}

	outlook.DepTime = "2500"
	if !errors.Is(outlook.Validate(), domain.ErrInvalidHHMM) {
		t.Fatal("expected invalid departure time")
	}

	outlook.DepTime = "0700"

	outlook.DepTimeWindowMinutes = 0
	if err := outlook.Validate(); err == nil {
		t.Fatal("expected window error")
	}

	stats := RouteStatsFilter{StartDate: "2025-01-01", EndDate: "2025-01-31", DaysOfWeek: []int{1, 2}}
	if err := stats.Validate(); err != nil {
		t.Fatalf("stats Validate: %v", err)
	}

	stats.EndDate = "2026-12-31"
	if err := stats.Validate(); err == nil {
		t.Fatal("expected span error")
	}

	stats.EndDate = "2024-01-01"
	if !errors.Is(stats.Validate(), domain.ErrStatsDateOrder) {
		t.Fatal("expected end before start")
	}

	stats.EndDate = "2025-01-31"

	stats.DaysOfWeek = []int{8}
	if !errors.Is(stats.Validate(), domain.ErrInvalidWeekday) {
		t.Fatal("expected invalid weekday")
	}
}

func TestDelayPercentilesMatchFilteredSubset(t *testing.T) {
	// percentile_cont(p) FILTER (WHERE pred) matches the same ordered-set
	// aggregate over that subset. Route stats and route outlook use it for
	// arrival and departure medians; outlook also uses it for arrival p90.
	// Cancelled, diverted, and null delays stay out of those populations.
	type flight struct {
		cancelled bool
		diverted  bool
		delayed   bool
		arr       *float64
		dep       *float64
	}

	f := func(v float64) *float64 { return &v }

	rows := []flight{
		{arr: f(-10), dep: f(-5)},
		{arr: f(0), dep: f(0)},
		{arr: f(5), dep: f(4)},
		{delayed: true, arr: f(20), dep: f(15)},
		{delayed: true, arr: f(40), dep: f(30)},
		{delayed: true, arr: f(60)},
		{cancelled: true, arr: f(999), dep: f(999)},
		{diverted: true, arr: f(50), dep: f(50)},
		{dep: f(8)},
	}

	arrived := func(row flight) bool { return !row.cancelled && !row.diverted }
	isDelayed := func(row flight) bool { return row.delayed }
	arrOf := func(row flight) *float64 { return row.arr }
	depOf := func(row flight) *float64 { return row.dep }

	values := func(include func(flight) bool, field func(flight) *float64) []float64 {
		out := make([]float64, 0)

		for _, row := range rows {
			if !include(row) {
				continue
			}

			v := field(row)
			if v == nil {
				continue
			}

			out = append(out, *v)
		}

		return out
	}

	tests := []struct {
		name   string
		p      float64
		values []float64
		want   float64
	}{
		{name: "median arrival", p: 0.5, values: values(arrived, arrOf), want: 12.5},
		{name: "median arrival when delayed", p: 0.5, values: values(isDelayed, arrOf), want: 40},
		{name: "median departure", p: 0.5, values: values(arrived, depOf), want: 6},
		{name: "median departure when delayed", p: 0.5, values: values(isDelayed, depOf), want: 22.5},
		{name: "p90 arrival", p: 0.9, values: values(arrived, arrOf), want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentileCont(tt.p, tt.values); got != tt.want {
				t.Fatalf("percentile_cont(%v) = %v, want %v (values %v)", tt.p, got, tt.want, tt.values)
			}
		})
	}

	arrivedArr := values(arrived, arrOf)
	if len(arrivedArr) != 6 {
		t.Fatalf("arrived arrival delays = %d, want 6", len(arrivedArr))
	}

	for _, row := range rows {
		if (!row.cancelled && !row.diverted) || row.arr == nil {
			continue
		}

		if containsFloat(arrivedArr, *row.arr) {
			t.Fatalf("excluded delay %v entered the arrival median", *row.arr)
		}
	}
}

func containsFloat(values []float64, want float64) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}

	return false
}

// classifyRouteStatsOnTime reports 1 when the flight is on time: not cancelled,
// then not diverted, then arr_del15 < 1. Cancelled takes priority over diverted,
// and diverted takes priority over delayed.

// percentileCont matches PostgreSQL percentile_cont: RN = 1 + p*(N-1), interpolating.
func percentileCont(p float64, values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	n := len(sorted)
	if n == 0 {
		return 0
	}

	if n == 1 {
		return sorted[0]
	}

	rn := 1 + p*float64(n-1)
	lo := int(math.Floor(rn))
	hi := int(math.Ceil(rn))

	if lo == hi {
		return sorted[lo-1]
	}

	return sorted[lo-1] + (rn-float64(lo))*(sorted[hi-1]-sorted[lo-1])
}
