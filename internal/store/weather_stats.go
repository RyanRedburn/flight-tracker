package store

import "github.com/RyanRedburn/flight-tracker/internal/model"

const (
	// WeatherMatchWindowMinutes is how far an observation may sit from the
	// scheduled timestamp and still match. The nearest observation inside the
	// window wins; ties break toward the earlier valid time.
	// Origin uses CRS departure (crs_dep_time on flight_date in the origin
	// station timezone). Destination uses that departure instant plus
	// crs_elapsed_time minutes (CRS block). crs_arr_time is not used, so
	// overnight flights are not pinned to the departure calendar date.
	WeatherMatchWindowMinutes = 30

	WeatherSideOrigin = "origin"
	WeatherSideDest   = "dest"
	WeatherUnmatched  = "UNMATCHED"

	// WeatherStatsRebuildLockKey is held for the whole truncate+insert.
	// WeatherStatsRebuildJobLockKey serializes queueing only, so a POST does
	// not block until that rebuild commits.
	WeatherStatsRebuildLockKey    = "rebuild_route_weather_stats"
	WeatherStatsRebuildJobLockKey = "queue_rebuild_route_weather_stats"
)

// adverseWeatherCategories is every stored category except VFR_FAIR and UNKNOWN.
// UNMATCHED is a rollup bucket, not a category, so it is absent here too.
var adverseWeatherCategories = adverseWeatherCategorySet()

// WeatherCategoryCount is one precomputed side/category rollup row summed for a filter.
type WeatherCategoryCount struct {
	Side        string
	Category    string
	OnTimeCount int
	Flights     int
}

// AssembleRouteWeatherStats turns per-side rollup counts into the API payload.
// UNMATCHED is flights_unmatched on that side and is not a category bucket.
// Every flight is counted on both sides, so origin and destination flight totals match.
func AssembleRouteWeatherStats(filter RouteStatsFilter, counts []WeatherCategoryCount) *model.RouteWeatherStats {
	out := emptyRouteWeatherStats(filter)
	origin := newWeatherSide()
	dest := newWeatherSide()

	for _, count := range counts {
		switch count.Side {
		case WeatherSideOrigin:
			origin.add(count)
		case WeatherSideDest:
			dest.add(count)
		}
	}

	out.OriginWeather = origin.stats()
	out.DestWeather = dest.stats()
	out.Flights = out.OriginWeather.Flights
	out.Analysis.SampleSize = out.Flights
	out.RoundForResponse()

	return out
}

func emptyRouteWeatherStats(filter RouteStatsFilter) *model.RouteWeatherStats {
	return &model.RouteWeatherStats{
		Filters: model.Filters{
			Origin:       filter.Origin,
			Dest:         filter.Dest,
			Carrier:      filter.Carrier,
			FlightNumber: filter.FlightNumber,
			DaysOfWeek:   filter.DaysOfWeek,
			StartDate:    filter.StartDate,
			EndDate:      filter.EndDate,
		},
		Analysis: model.Analysis{
			Start: filter.StartDate,
			End:   filter.EndDate,
		},
	}
}

type weatherCatCount struct {
	onTime  int
	flights int
}

type weatherSideAccum struct {
	flights   int
	unmatched int
	byCat     map[string]weatherCatCount
}

func newWeatherSide() *weatherSideAccum {
	order := model.WeatherCategoryOrder()
	byCat := make(map[string]weatherCatCount, len(order))

	for _, category := range order {
		byCat[category] = weatherCatCount{}
	}

	return &weatherSideAccum{byCat: byCat}
}

func (a *weatherSideAccum) add(count WeatherCategoryCount) {
	if count.Flights < 1 {
		return
	}

	if count.Category == WeatherUnmatched {
		a.flights += count.Flights
		a.unmatched += count.Flights

		return
	}

	slot, ok := a.byCat[count.Category]
	if !ok {
		return
	}

	slot.onTime += count.OnTimeCount
	slot.flights += count.Flights
	a.byCat[count.Category] = slot
	a.flights += count.Flights
}

// AssembleRouteOutlookWeather folds per-side rollup counts into the outlook modifier.
// Fair is VFR_FAIR. Adverse is every other category except UNKNOWN.
// UNMATCHED and unknown category strings are excluded.
// Confidence uses the thinner of the two groups and the outlook sample minimum:
// absent fair or adverse flights are unknown, and a short group is low.
// Rates stay on a low-confidence side. A missing group is a null rate, so adverse_delta is null.
func AssembleRouteOutlookWeather(counts []WeatherCategoryCount) model.RouteOutlookWeather {
	origin := &outlookWeatherAccum{}
	dest := &outlookWeatherAccum{}

	for _, count := range counts {
		switch count.Side {
		case WeatherSideOrigin:
			origin.add(count)
		case WeatherSideDest:
			dest.add(count)
		}
	}

	out := model.RouteOutlookWeather{
		Origin: origin.side(),
		Dest:   dest.side(),
	}
	out.RoundForResponse()

	return out
}

type outlookWeatherAccum struct {
	fairOnTime     int
	fairFlights    int
	adverseOnTime  int
	adverseFlights int
}

func (a *outlookWeatherAccum) add(count WeatherCategoryCount) {
	if count.Flights < 1 {
		return
	}

	if count.Category == model.WeatherCategoryVFRFair {
		a.fairOnTime += count.OnTimeCount
		a.fairFlights += count.Flights

		return
	}

	if _, ok := adverseWeatherCategories[count.Category]; !ok {
		return
	}

	a.adverseOnTime += count.OnTimeCount
	a.adverseFlights += count.Flights
}

func (a *outlookWeatherAccum) side() model.RouteOutlookWeatherSide {
	return model.RouteOutlookWeatherSide{
		FairOnTimeRate:    sampleRatePtr(a.fairOnTime, a.fairFlights),
		AdverseOnTimeRate: sampleRatePtr(a.adverseOnTime, a.adverseFlights),
		AdverseFlights:    a.adverseFlights,
		Confidence:        weatherSideConfidence(a.fairFlights, a.adverseFlights),
	}
}

func adverseWeatherCategorySet() map[string]struct{} {
	set := make(map[string]struct{}, len(model.WeatherCategoryOrder()))

	for _, category := range model.WeatherCategoryOrder() {
		if category == model.WeatherCategoryVFRFair || category == model.WeatherCategoryUnknown {
			continue
		}

		set[category] = struct{}{}
	}

	return set
}

func weatherSideConfidence(fairFlights, adverseFlights int) string {
	sample := fairFlights
	if adverseFlights < sample {
		sample = adverseFlights
	}

	return model.SeasonalConfidence(sample, MinOutlookSampleSize)
}

func sampleRatePtr(onTime, flights int) *float64 {
	if flights <= 0 {
		return nil
	}

	v := rate(onTime, flights)

	return &v
}

func (a *weatherSideAccum) stats() model.WeatherSideStats {
	order := model.WeatherCategoryOrder()
	out := model.WeatherSideStats{
		Flights:          a.flights,
		FlightsUnmatched: a.unmatched,
		Categories:       make([]model.WeatherCategoryStat, 0, len(order)),
	}

	for _, category := range order {
		count := a.byCat[category]
		out.Categories = append(out.Categories, model.WeatherCategoryStat{
			Category:   category,
			Flights:    count.flights,
			OnTimeRate: rate(count.onTime, count.flights),
		})
	}

	return out
}
