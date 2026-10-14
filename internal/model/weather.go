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

// RouteOutlookWeather is informational fair versus adverse on-time history.
// It does not change connection status or confidence.
// Fair is VFR_FAIR. Adverse is every other category except UNKNOWN. Unmatched flights are excluded.
type RouteOutlookWeather struct {
	Origin RouteOutlookWeatherSide `json:"origin"`
	Dest   RouteOutlookWeatherSide `json:"dest"`
}

// RouteOutlookWeatherSide is one airport's fair versus adverse split.
type RouteOutlookWeatherSide struct {
	// FairOnTimeRate is null when this side has no VFR_FAIR flights.
	FairOnTimeRate *float64 `json:"fair_on_time_rate"`
	// AdverseOnTimeRate is null when this side has no adverse flights.
	AdverseOnTimeRate *float64 `json:"adverse_on_time_rate"`
	// AdverseDelta is adverse_on_time_rate minus fair_on_time_rate, or null unless both rates exist.
	AdverseDelta *float64 `json:"adverse_delta"`
	// AdverseFlights counts adverse-category flights. UNKNOWN and unmatched flights are excluded.
	AdverseFlights int `json:"adverse_flights"`
	// Confidence is unknown when fair or adverse flights are absent, and low when the thinner group is below the outlook minimum. A thin sample keeps its rates.
	Confidence string `json:"confidence" enums:"high,low,unknown"`
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

// RoundForResponse rounds rates to two decimal places.
// adverse_delta is recomputed from those rounded rates so it matches the published pair.
func (w *RouteOutlookWeather) RoundForResponse() {
	w.Origin.round()
	w.Dest.round()
}

func (s *RouteOutlookWeatherSide) round() {
	s.FairOnTimeRate = roundRatePtr(s.FairOnTimeRate)
	s.AdverseOnTimeRate = roundRatePtr(s.AdverseOnTimeRate)

	if s.FairOnTimeRate == nil || s.AdverseOnTimeRate == nil {
		s.AdverseDelta = nil
		return
	}

	delta := roundRate(*s.AdverseOnTimeRate - *s.FairOnTimeRate)
	s.AdverseDelta = &delta
}
