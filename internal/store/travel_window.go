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
	out.BestMonths, out.WorstMonths, out.MonthComparison = rankTravelMonths(months, monthMin)
	out.BestDays = pickTravelWindowBuckets(days, dayBucketKey, dayBucketRate, dayBucketFlights, dayMin, true)
	out.WorstDays = pickTravelWindowBuckets(days, dayBucketKey, dayBucketRate, dayBucketFlights, dayMin, false)
	out.BestHours = pickTravelWindowBuckets(hours, hourBucketKey, hourBucketRate, hourBucketFlights, hourMin, true)
	out.WorstHours = pickTravelWindowBuckets(hours, hourBucketKey, hourBucketRate, hourBucketFlights, hourMin, false)

	out.RoundForResponse()
	labelSeasonalBuckets(out, dayMin, hourMin)

	return out
}

// labelSeasonalBuckets sets confidence and reliability from the rounded on-time rate.
// Best and worst copies use the same rule as the full series.
// Month confidence uses the history bands, so a month can clear the eligibility floor and still be low.
// Day and hour confidence stays high or low from the eligibility floor.
// reliability unknown with confidence low is a thin bucket. A medium or high rate below the typical band is unreliable.
// Buckets with no flights are omitted earlier, so they are not published as a zero rate.
func labelSeasonalBuckets(out *model.RouteTravelWindows, dayMin, hourMin int) {
	for i := range out.ByMonth {
		out.ByMonth[i].Confidence, out.ByMonth[i].Reliability = monthDecision(out.ByMonth[i].Flights, out.ByMonth[i].OnTimeRate)
	}

	for i := range out.BestMonths {
		out.BestMonths[i].Confidence, out.BestMonths[i].Reliability = monthDecision(out.BestMonths[i].Flights, out.BestMonths[i].OnTimeRate)
	}

	for i := range out.WorstMonths {
		out.WorstMonths[i].Confidence, out.WorstMonths[i].Reliability = monthDecision(out.WorstMonths[i].Flights, out.WorstMonths[i].OnTimeRate)
	}

	for i := range out.ByDayOfWeek {
		out.ByDayOfWeek[i].Confidence, out.ByDayOfWeek[i].Reliability = model.SeasonalDecision(out.ByDayOfWeek[i].Flights, out.ByDayOfWeek[i].OnTimeRate, dayMin)
	}

	for i := range out.BestDays {
		out.BestDays[i].Confidence, out.BestDays[i].Reliability = model.SeasonalDecision(out.BestDays[i].Flights, out.BestDays[i].OnTimeRate, dayMin)
	}

	for i := range out.WorstDays {
		out.WorstDays[i].Confidence, out.WorstDays[i].Reliability = model.SeasonalDecision(out.WorstDays[i].Flights, out.WorstDays[i].OnTimeRate, dayMin)
	}

	for i := range out.ByHour {
		out.ByHour[i].Confidence, out.ByHour[i].Reliability = model.SeasonalDecision(out.ByHour[i].Flights, out.ByHour[i].OnTimeRate, hourMin)
	}

	for i := range out.BestHours {
		out.BestHours[i].Confidence, out.BestHours[i].Reliability = model.SeasonalDecision(out.BestHours[i].Flights, out.BestHours[i].OnTimeRate, hourMin)
	}

	for i := range out.WorstHours {
		out.WorstHours[i].Confidence, out.WorstHours[i].Reliability = model.SeasonalDecision(out.WorstHours[i].Flights, out.WorstHours[i].OnTimeRate, hourMin)
	}
}

func monthDecision(flights int, onTimeRate float64) (confidence, reliability string) {
	confidence = model.CountConfidence(flights, model.HistoryLowSampleBelow, model.HistoryHighSampleAt)
	reliability = model.SeasonalReliability(onTimeRate, confidence)

	return confidence, reliability
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

// rankTravelMonths keeps months tied with the highest-rate eligible month, and
// months tied with the lowest-rate one, only when that highest-rate month differs
// at 95% from at least one other. Otherwise the lists are empty and the comparison
// is too close to call. One eligible month is still returned on both lists and is
// not a comparison. The floor is the caller's minSample, not the confidence band.
func rankTravelMonths(months []model.TravelWindowMonthBucket, minSample int) (best, worst []model.TravelWindowMonthBucket, comparison string) {
	eligible := make([]model.TravelWindowMonthBucket, 0, len(months))

	for _, month := range months {
		if month.Flights >= minSample {
			eligible = append(eligible, month)
		}
	}

	if len(eligible) < 2 {
		return pickTravelWindowBuckets(months, monthBucketKey, monthBucketRate, monthBucketFlights, minSample, true),
			pickTravelWindowBuckets(months, monthBucketKey, monthBucketRate, monthBucketFlights, minSample, false),
			""
	}

	sortTravelMonths(eligible, true)

	if !monthBeatsAny(eligible[0], eligible[1:]) {
		return []model.TravelWindowMonthBucket{}, []model.TravelWindowMonthBucket{}, model.ComparisonTooCloseToCall
	}

	best = monthsTied(eligible[0], eligible)
	worst = monthsTied(eligible[len(eligible)-1], eligible)
	sortTravelMonths(worst, false)

	return best, worst, model.ComparisonRanked
}

func sortTravelMonths(months []model.TravelWindowMonthBucket, descending bool) {
	sort.SliceStable(months, func(i, j int) bool {
		left := model.ComparisonRate(months[i].OnTimeRate, months[i].OnTimeCount, months[i].Flights)
		right := model.ComparisonRate(months[j].OnTimeRate, months[j].OnTimeCount, months[j].Flights)

		if left != right {
			if descending {
				return left > right
			}

			return left < right
		}

		if months[i].Flights != months[j].Flights {
			return months[i].Flights > months[j].Flights
		}

		return months[i].Month < months[j].Month
	})
}

func monthBeatsAny(leader model.TravelWindowMonthBucket, others []model.TravelWindowMonthBucket) bool {
	for _, month := range others {
		if monthsDiffer(leader, month) {
			return true
		}
	}

	return false
}

func monthsTied(anchor model.TravelWindowMonthBucket, months []model.TravelWindowMonthBucket) []model.TravelWindowMonthBucket {
	tied := make([]model.TravelWindowMonthBucket, 0, len(months))

	for _, month := range months {
		if !monthsDiffer(anchor, month) {
			tied = append(tied, month)
		}
	}

	return tied
}

func monthsDiffer(a, b model.TravelWindowMonthBucket) bool {
	return model.ProportionsDiffer(
		model.ComparisonRate(a.OnTimeRate, a.OnTimeCount, a.Flights),
		a.Flights,
		model.ComparisonRate(b.OnTimeRate, b.OnTimeCount, b.Flights),
		b.Flights,
	)
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
