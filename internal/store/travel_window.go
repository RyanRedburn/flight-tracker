package store

import (
	"sort"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	TravelWindowLookbackYears = 2
	TravelWindowBestWorstK    = 3
	TravelWindowHourFloor     = 30
	TravelWindowDayFloor      = 50
	TravelWindowMonthFloor    = 50

	TravelWindowGrainMonth     = "month"
	TravelWindowGrainDayOfWeek = "day_of_week"
	TravelWindowGrainHour      = "hour"
	TravelWindowRebuildLockKey = "rebuild_route_travel_windows"
)

type RouteTravelWindowsFilter struct {
	Origin  string
	Dest    string
	Carrier string
}

type TravelWindowBucketCount struct {
	Grain       string
	Bucket      int
	OnTimeCount int
	Flights     int
}

// TravelWindowBounds returns the inclusive analysis window of up to the last
// two years ending at maxDate. If history is shorter, start is minDate.
// Rebuild SQL uses the same rule: GREATEST(min_date, (max_date - INTERVAL '2 years')::date).
func TravelWindowBounds(minDate, maxDate time.Time) (start, end time.Time) {
	start = maxDate.AddDate(-TravelWindowLookbackYears, 0, 0)
	if minDate.After(start) {
		start = minDate
	}

	return start, maxDate
}

// TravelWindowMinSample is max(floor, ceil(0.01 * windowFlights)).
func TravelWindowMinSample(floor, windowFlights int) int {
	share := 0
	if windowFlights > 0 {
		share = (windowFlights + 99) / 100
	}

	if floor > share {
		return floor
	}

	return share
}

func AssembleRouteTravelWindows(
	origin, dest, carrier, windowStart, windowEnd string,
	windowFlights int,
	counts []TravelWindowBucketCount,
) *model.RouteTravelWindows {
	out := emptyRouteTravelWindows(origin, dest, carrier, windowStart, windowEnd)

	months := make([]model.TravelWindowMonthBucket, 0)
	days := make([]model.TravelWindowDayBucket, 0)
	hours := make([]model.TravelWindowHourBucket, 0)

	for _, count := range counts {
		if count.Flights < 1 {
			continue
		}

		onTimeRate := rate(count.OnTimeCount, count.Flights)

		switch count.Grain {
		case TravelWindowGrainMonth:
			months = append(months, model.TravelWindowMonthBucket{
				Month:      count.Bucket,
				OnTimeRate: onTimeRate,
				Flights:    count.Flights,
			})
		case TravelWindowGrainDayOfWeek:
			days = append(days, model.TravelWindowDayBucket{
				DayOfWeek:  count.Bucket,
				OnTimeRate: onTimeRate,
				Flights:    count.Flights,
			})
		case TravelWindowGrainHour:
			hours = append(hours, model.TravelWindowHourBucket{
				Hour:       count.Bucket,
				OnTimeRate: onTimeRate,
				Flights:    count.Flights,
			})
		}
	}

	sort.Slice(months, func(i, j int) bool { return months[i].Month < months[j].Month })
	sort.Slice(days, func(i, j int) bool { return days[i].DayOfWeek < days[j].DayOfWeek })
	sort.Slice(hours, func(i, j int) bool { return hours[i].Hour < hours[j].Hour })

	monthMin := TravelWindowMinSample(TravelWindowMonthFloor, windowFlights)
	dayMin := TravelWindowMinSample(TravelWindowDayFloor, windowFlights)
	hourMin := TravelWindowMinSample(TravelWindowHourFloor, windowFlights)

	out.ByMonth = months
	out.ByDayOfWeek = days
	out.ByHour = hours
	out.BestMonths = pickTravelWindowBuckets(months, monthBucketKey, monthBucketRate, monthBucketFlights, monthMin, true)
	out.WorstMonths = pickTravelWindowBuckets(months, monthBucketKey, monthBucketRate, monthBucketFlights, monthMin, false)
	out.BestDays = pickTravelWindowBuckets(days, dayBucketKey, dayBucketRate, dayBucketFlights, dayMin, true)
	out.WorstDays = pickTravelWindowBuckets(days, dayBucketKey, dayBucketRate, dayBucketFlights, dayMin, false)
	out.BestHours = pickTravelWindowBuckets(hours, hourBucketKey, hourBucketRate, hourBucketFlights, hourMin, true)
	out.WorstHours = pickTravelWindowBuckets(hours, hourBucketKey, hourBucketRate, hourBucketFlights, hourMin, false)

	out.RoundForResponse()

	return out
}

func emptyRouteTravelWindows(origin, dest, carrier, windowStart, windowEnd string) *model.RouteTravelWindows {
	return &model.RouteTravelWindows{
		Origin:      origin,
		Dest:        dest,
		Carrier:     carrier,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		ByMonth:     []model.TravelWindowMonthBucket{},
		ByDayOfWeek: []model.TravelWindowDayBucket{},
		ByHour:      []model.TravelWindowHourBucket{},
		BestMonths:  []model.TravelWindowMonthBucket{},
		WorstMonths: []model.TravelWindowMonthBucket{},
		BestDays:    []model.TravelWindowDayBucket{},
		WorstDays:   []model.TravelWindowDayBucket{},
		BestHours:   []model.TravelWindowHourBucket{},
		WorstHours:  []model.TravelWindowHourBucket{},
	}
}

func pickTravelWindowBuckets[T any](
	items []T,
	key func(T) int,
	onTimeRate func(T) float64,
	flights func(T) int,
	minSample int,
	best bool,
) []T {
	eligible := make([]T, 0, len(items))
	for _, item := range items {
		if flights(item) >= minSample {
			eligible = append(eligible, item)
		}
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		if onTimeRate(eligible[i]) != onTimeRate(eligible[j]) {
			if best {
				return onTimeRate(eligible[i]) > onTimeRate(eligible[j])
			}

			return onTimeRate(eligible[i]) < onTimeRate(eligible[j])
		}

		if flights(eligible[i]) != flights(eligible[j]) {
			return flights(eligible[i]) > flights(eligible[j])
		}

		return key(eligible[i]) < key(eligible[j])
	})

	top, _ := splitTopN(eligible, TravelWindowBestWorstK)
	if top == nil {
		return []T{}
	}

	return top
}

func monthBucketKey(b model.TravelWindowMonthBucket) int { return b.Month }

func monthBucketRate(b model.TravelWindowMonthBucket) float64 { return b.OnTimeRate }

func monthBucketFlights(b model.TravelWindowMonthBucket) int { return b.Flights }

func dayBucketKey(b model.TravelWindowDayBucket) int { return b.DayOfWeek }

func dayBucketRate(b model.TravelWindowDayBucket) float64 { return b.OnTimeRate }

func dayBucketFlights(b model.TravelWindowDayBucket) int { return b.Flights }

func hourBucketKey(b model.TravelWindowHourBucket) int { return b.Hour }

func hourBucketRate(b model.TravelWindowHourBucket) float64 { return b.OnTimeRate }

func hourBucketFlights(b model.TravelWindowHourBucket) int { return b.Flights }
