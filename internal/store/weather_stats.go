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
	out.RoundForResponse()

	return out
}

func emptyRouteWeatherStats(filter RouteStatsFilter) *model.RouteWeatherStats {
	days := append([]int(nil), filter.DaysOfWeek...)
	if days == nil {
		days = []int{}
	}

	return &model.RouteWeatherStats{
		Origin:    filter.Origin,
		Dest:      filter.Dest,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		Filters: model.RouteStatsFilters{
			Carrier:      filter.Carrier,
			FlightNumber: filter.FlightNumber,
			DaysOfWeek:   days,
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
