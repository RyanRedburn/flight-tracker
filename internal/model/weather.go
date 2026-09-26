package model

const (
	WeatherCategoryThunder = "THUNDER"
	WeatherCategoryLIFR    = "LIFR"
	WeatherCategoryIFR     = "IFR"
	WeatherCategoryMVFR    = "MVFR"
	WeatherCategoryWindy   = "WINDY"
	WeatherCategoryPrecip  = "PRECIP"
	WeatherCategoryVFRFair = "VFR_FAIR"
	WeatherCategoryUnknown = "UNKNOWN"
)

// WeatherCategoryStat is on-time performance for one METAR category.
type WeatherCategoryStat struct {
	Category   string  `json:"category"`
	Flights    int     `json:"flights"`
	OnTimeRate float64 `json:"on_time_rate"`
}

// WeatherSideStats is the category breakdown for origin or destination weather.
// flights includes unmatched flights. flights_unmatched is never folded into VFR_FAIR.
type WeatherSideStats struct {
	Flights          int                   `json:"flights"`
	FlightsUnmatched int                   `json:"flights_unmatched"`
	Categories       []WeatherCategoryStat `json:"categories"`
}

// RouteWeatherStats is on-time performance by observed METAR category for a route.
type RouteWeatherStats struct {
	Origin        string            `json:"origin"`
	Dest          string            `json:"dest"`
	StartDate     string            `json:"start_date"`
	EndDate       string            `json:"end_date"`
	Filters       RouteStatsFilters `json:"filters"`
	Flights       int               `json:"flights"`
	OriginWeather WeatherSideStats  `json:"origin_weather"`
	DestWeather   WeatherSideStats  `json:"dest_weather"`
}

// WeatherCategoryOrder is the first-match category priority, then UNKNOWN.
func WeatherCategoryOrder() []string {
	return []string{
		WeatherCategoryThunder,
		WeatherCategoryLIFR,
		WeatherCategoryIFR,
		WeatherCategoryMVFR,
		WeatherCategoryWindy,
		WeatherCategoryPrecip,
		WeatherCategoryVFRFair,
		WeatherCategoryUnknown,
	}
}

// RoundForResponse rounds category on-time rates to two decimal places.
func (s *RouteWeatherStats) RoundForResponse() {
	roundWeatherSide(&s.OriginWeather)
	roundWeatherSide(&s.DestWeather)
}

func roundWeatherSide(side *WeatherSideStats) {
	for i := range side.Categories {
		side.Categories[i].OnTimeRate = roundRate(side.Categories[i].OnTimeRate)
	}
}
