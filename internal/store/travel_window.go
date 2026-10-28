package store

import (
	"sort"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	TravelWindowLookbackYears = 2
	TravelWindowBestWorstK    = 3

	TravelWindowGrainMonth     = "month"
	TravelWindowGrainDayOfWeek = "day_of_week"
	TravelWindowGrainHour      = "hour"

	// TravelWindowRebuildLockKey is held for the whole truncate+insert.
	// TravelWindowRebuildJobLockKey serializes queueing only, so a POST does
	// not block until that rebuild commits.
	TravelWindowRebuildLockKey    = "rebuild_route_travel_windows"
	TravelWindowRebuildJobLockKey = "queue_rebuild_route_travel_windows"
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

func AssembleRouteTravelWindows(
	origin, dest, carrier, windowStart, windowEnd string,
	windowFlights int,
	counts []TravelWindowBucketCount,
) *model.RouteTravelWindows {
	out := emptyRouteTravelWindows(origin, dest, carrier, windowStart, windowEnd, windowFlights)

	months := make([]model.TravelWindowMonthBucket, 0)
	days := make([]model.TravelWindowDayBucket, 0)
	hours := make([]model.TravelWindowHourBucket, 0)

	for _, count := range counts {
		if count.Flights < 1 {
			continue
		}

		onTime := count.OnTimeCount
		onTimeRate := rate(onTime, count.Flights)

		switch count.Grain {
		case TravelWindowGrainMonth:
			months = append(months, model.TravelWindowMonthBucket{
				Month:       count.Bucket,
				OnTimeRate:  onTimeRate,
				Flights:     count.Flights,
				OnTimeCount: &onTime,
			})
		case TravelWindowGrainDayOfWeek:
			days = append(days, model.TravelWindowDayBucket{
				DayOfWeek:   count.Bucket,
				OnTimeRate:  onTimeRate,
				Flights:     count.Flights,
				OnTimeCount: &onTime,
			})
		case TravelWindowGrainHour:
			hours = append(hours, model.TravelWindowHourBucket{
				Hour:        count.Bucket,
				OnTimeRate:  onTimeRate,
				Flights:     count.Flights,
				OnTimeCount: &onTime,
			})
		}
	}

	sort.Slice(months, func(i, j int) bool { return months[i].Month < months[j].Month })
	sort.Slice(days, func(i, j int) bool { return days[i].DayOfWeek < days[j].DayOfWeek })
	sort.Slice(hours, func(i, j int) bool { return hours[i].Hour < hours[j].Hour })

	out.ByMonth = months
	out.ByDayOfWeek = days
	out.ByHour = hours
	out.BestMonths, out.WorstMonths, out.MonthComparison = rankTravelBuckets(months, monthBucketKey, monthBucketRate, monthBucketCount, monthBucketFlights)
	out.BestDays, out.WorstDays, out.DayComparison = rankTravelBuckets(days, dayBucketKey, dayBucketRate, dayBucketCount, dayBucketFlights)
	out.BestHours, out.WorstHours, out.HourComparison = rankTravelBuckets(hours, hourBucketKey, hourBucketRate, hourBucketCount, hourBucketFlights)

	out.RoundForResponse()
	labelSeasonalBuckets(out)

	return out
}

// labelSeasonalBuckets sets confidence and reliability from the rounded on-time rate.
// Best and worst copies use the same rule as the full series.
// Every grain uses the outlook bands. reliability unknown with confidence low is a thin bucket.
// A medium or high rate below the typical band is unreliable.
// Buckets with no flights are omitted earlier, so they are not published as a zero rate.
func labelSeasonalBuckets(out *model.RouteTravelWindows) {
	for i := range out.ByMonth {
		out.ByMonth[i].Confidence, out.ByMonth[i].Reliability = model.SeasonalDecision(out.ByMonth[i].Flights, out.ByMonth[i].OnTimeRate)
	}

	for i := range out.BestMonths {
		out.BestMonths[i].Confidence, out.BestMonths[i].Reliability = model.SeasonalDecision(out.BestMonths[i].Flights, out.BestMonths[i].OnTimeRate)
	}

	for i := range out.WorstMonths {
		out.WorstMonths[i].Confidence, out.WorstMonths[i].Reliability = model.SeasonalDecision(out.WorstMonths[i].Flights, out.WorstMonths[i].OnTimeRate)
	}

	for i := range out.ByDayOfWeek {
		out.ByDayOfWeek[i].Confidence, out.ByDayOfWeek[i].Reliability = model.SeasonalDecision(out.ByDayOfWeek[i].Flights, out.ByDayOfWeek[i].OnTimeRate)
	}

	for i := range out.BestDays {
		out.BestDays[i].Confidence, out.BestDays[i].Reliability = model.SeasonalDecision(out.BestDays[i].Flights, out.BestDays[i].OnTimeRate)
	}

	for i := range out.WorstDays {
		out.WorstDays[i].Confidence, out.WorstDays[i].Reliability = model.SeasonalDecision(out.WorstDays[i].Flights, out.WorstDays[i].OnTimeRate)
	}

	for i := range out.ByHour {
		out.ByHour[i].Confidence, out.ByHour[i].Reliability = model.SeasonalDecision(out.ByHour[i].Flights, out.ByHour[i].OnTimeRate)
	}

	for i := range out.BestHours {
		out.BestHours[i].Confidence, out.BestHours[i].Reliability = model.SeasonalDecision(out.BestHours[i].Flights, out.BestHours[i].OnTimeRate)
	}

	for i := range out.WorstHours {
		out.WorstHours[i].Confidence, out.WorstHours[i].Reliability = model.SeasonalDecision(out.WorstHours[i].Flights, out.WorstHours[i].OnTimeRate)
	}
}

func emptyRouteTravelWindows(origin, dest, carrier, windowStart, windowEnd string, windowFlights int) *model.RouteTravelWindows {
	return &model.RouteTravelWindows{
		Filters: model.Filters{
			Origin:  origin,
			Dest:    dest,
			Carrier: carrier,
		},
		Analysis: model.Analysis{
			Start:      windowStart,
			End:        windowEnd,
			SampleSize: windowFlights,
		},
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

// rankTravelBuckets keeps buckets tied with the leader, only when that leader
// differs at 95% from at least one other. The leader is the highest 95% Wilson
// lower bound. Worst buckets are the beaten bucket with the lowest Wilson upper
// bound, plus buckets tied with that one. Each list is capped at
// TravelWindowBestWorstK, then shown by on-time rate, highest first.
// Otherwise the lists are empty and the comparison is too close to call.
// One bucket leaves both lists empty and comparison single.
// A bucket with flights is rankable. Ties use ProportionsDiffer.
func rankTravelBuckets[T any](
	items []T,
	key func(T) int,
	onTimeRate func(T) float64,
	onTimeCount func(T) *int,
	flights func(T) int,
) (best, worst []T, comparison string) {
	if len(items) == 0 {
		return []T{}, []T{}, ""
	}

	if len(items) == 1 {
		return []T{}, []T{}, model.ComparisonSingle
	}

	ordered := append([]T(nil), items...)
	sortByWilsonLower(ordered, key, onTimeRate, onTimeCount, flights)

	leader := ordered[0]
	beaten := make([]T, 0, len(ordered)-1)

	for _, item := range ordered[1:] {
		if travelBucketsDiffer(leader, item, onTimeRate, onTimeCount, flights) {
			beaten = append(beaten, item)
		}
	}

	if len(beaten) == 0 {
		return []T{}, []T{}, model.ComparisonTooCloseToCall
	}

	tied := make([]T, 0, len(ordered))

	for _, item := range ordered {
		if !travelBucketsDiffer(leader, item, onTimeRate, onTimeCount, flights) {
			tied = append(tied, item)
		}
	}

	best = capTravelBuckets(tied)
	sortByRateDesc(best, key, onTimeRate, onTimeCount, flights)

	sortByWilsonUpper(beaten, key, onTimeRate, onTimeCount, flights)

	anchor := beaten[0]
	worstTied := make([]T, 0, len(beaten))

	for _, item := range beaten {
		if !travelBucketsDiffer(anchor, item, onTimeRate, onTimeCount, flights) {
			worstTied = append(worstTied, item)
		}
	}

	sortByWilsonUpper(worstTied, key, onTimeRate, onTimeCount, flights)

	worst = capTravelBuckets(worstTied)
	sortByRateDesc(worst, key, onTimeRate, onTimeCount, flights)

	return best, worst, model.ComparisonRanked
}

func capTravelBuckets[T any](items []T) []T {
	if len(items) > TravelWindowBestWorstK {
		items = items[:TravelWindowBestWorstK]
	}

	return append([]T(nil), items...)
}

func sortByWilsonLower[T any](
	items []T,
	key func(T) int,
	onTimeRate func(T) float64,
	onTimeCount func(T) *int,
	flights func(T) int,
) {
	sort.SliceStable(items, func(i, j int) bool {
		return wilsonLess(items[i], items[j], key, onTimeRate, onTimeCount, flights, false)
	})
}

func sortByRateDesc[T any](
	items []T,
	key func(T) int,
	onTimeRate func(T) float64,
	onTimeCount func(T) *int,
	flights func(T) int,
) {
	sort.SliceStable(items, func(i, j int) bool {
		left := travelBucketRate(items[i], onTimeRate, onTimeCount, flights)
		right := travelBucketRate(items[j], onTimeRate, onTimeCount, flights)

		if left != right {
			return left > right
		}

		if flights(items[i]) != flights(items[j]) {
			return flights(items[i]) > flights(items[j])
		}

		return key(items[i]) < key(items[j])
	})
}

func sortByWilsonUpper[T any](
	items []T,
	key func(T) int,
	onTimeRate func(T) float64,
	onTimeCount func(T) *int,
	flights func(T) int,
) {
	sort.SliceStable(items, func(i, j int) bool {
		return wilsonLess(items[i], items[j], key, onTimeRate, onTimeCount, flights, true)
	})
}

// wilsonLess orders a higher Wilson lower bound first, or a lower Wilson upper bound first.
// Equal bounds break by more flights, then the smaller key.
func wilsonLess[T any](
	a, b T,
	key func(T) int,
	onTimeRate func(T) float64,
	onTimeCount func(T) *int,
	flights func(T) int,
	upper bool,
) bool {
	aLower, aUpper := bucketWilson(a, onTimeRate, onTimeCount, flights)
	bLower, bUpper := bucketWilson(b, onTimeRate, onTimeCount, flights)
	left, right := aLower, bLower

	if upper {
		left, right = aUpper, bUpper
	}

	if left != right {
		if upper {
			return left < right
		}

		return left > right
	}

	if flights(a) != flights(b) {
		return flights(a) > flights(b)
	}

	return key(a) < key(b)
}

func bucketWilson[T any](item T, onTimeRate func(T) float64, onTimeCount func(T) *int, flights func(T) int) (lower, upper float64) {
	return model.WilsonBounds(travelBucketRate(item, onTimeRate, onTimeCount, flights), nil, flights(item))
}

func travelBucketsDiffer[T any](
	a, b T,
	onTimeRate func(T) float64,
	onTimeCount func(T) *int,
	flights func(T) int,
) bool {
	return model.ProportionsDiffer(
		travelBucketRate(a, onTimeRate, onTimeCount, flights),
		flights(a),
		travelBucketRate(b, onTimeRate, onTimeCount, flights),
		flights(b),
	)
}

func travelBucketRate[T any](item T, onTimeRate func(T) float64, onTimeCount func(T) *int, flights func(T) int) float64 {
	return model.ComparisonRate(onTimeRate(item), onTimeCount(item), flights(item))
}

func monthBucketKey(b model.TravelWindowMonthBucket) int { return b.Month }

func monthBucketRate(b model.TravelWindowMonthBucket) float64 { return b.OnTimeRate }

func monthBucketCount(b model.TravelWindowMonthBucket) *int { return b.OnTimeCount }

func monthBucketFlights(b model.TravelWindowMonthBucket) int { return b.Flights }

func dayBucketKey(b model.TravelWindowDayBucket) int { return b.DayOfWeek }

func dayBucketRate(b model.TravelWindowDayBucket) float64 { return b.OnTimeRate }

func dayBucketCount(b model.TravelWindowDayBucket) *int { return b.OnTimeCount }

func dayBucketFlights(b model.TravelWindowDayBucket) int { return b.Flights }

func hourBucketKey(b model.TravelWindowHourBucket) int { return b.Hour }

func hourBucketRate(b model.TravelWindowHourBucket) float64 { return b.OnTimeRate }

func hourBucketCount(b model.TravelWindowHourBucket) *int { return b.OnTimeCount }

func hourBucketFlights(b model.TravelWindowHourBucket) int { return b.Flights }
