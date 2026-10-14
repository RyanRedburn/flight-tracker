package model

import "math"

type DelayCausesAvgMinutes struct {
	Carrier      float64 `json:"carrier"`
	Weather      float64 `json:"weather"`
	NAS          float64 `json:"nas"`
	Security     float64 `json:"security"`
	LateAircraft float64 `json:"late_aircraft"`
}

type DelayCausesShare struct {
	Carrier      float64 `json:"carrier"`
	Weather      float64 `json:"weather"`
	NAS          float64 `json:"nas"`
	Security     float64 `json:"security"`
	LateAircraft float64 `json:"late_aircraft"`
	Unattributed float64 `json:"unattributed"`
}

type AirportCount struct {
	Airport string `json:"airport"`
	Count   int    `json:"count"`
}

// CarrierOnTime is per-marketing-carrier on-time performance for a route.
type CarrierOnTime struct {
	Carrier    string  `json:"carrier"`
	OnTimeRate float64 `json:"on_time_rate"`
	Flights    int     `json:"flights"`
}

type RouteStats struct {
	Filters                       Filters               `json:"filters"`
	Analysis                      Analysis              `json:"analysis"`
	Flights                       int                   `json:"flights"`
	OnTime                        int                   `json:"on_time"`
	Delayed                       int                   `json:"delayed"`
	Cancelled                     int                   `json:"cancelled"`
	Diverted                      int                   `json:"diverted"`
	OnTimeRate                    float64               `json:"on_time_rate"`
	DelayRate                     float64               `json:"delay_rate"`
	CancellationRate              float64               `json:"cancellation_rate"`
	DiversionRate                 float64               `json:"diversion_rate"`
	AvgArrivalDelayMinutes        float64               `json:"avg_arrival_delay_minutes"`
	MedianArrivalDelayMinutes     float64               `json:"median_arrival_delay_minutes"`
	AvgArrivalDelayWhenDelayed    float64               `json:"avg_arrival_delay_when_delayed"`
	MedianArrivalDelayWhenDelayed float64               `json:"median_arrival_delay_when_delayed"`
	AvgDepartureDelayMinutes      float64               `json:"avg_departure_delay_minutes"`
	MedianDepartureDelayMinutes   float64               `json:"median_departure_delay_minutes"`
	AvgDepartureDelayWhenDelayed  float64               `json:"avg_departure_delay_when_delayed"`
	DelayCausesAvgMinutes         DelayCausesAvgMinutes `json:"delay_causes_avg_minutes"`
	DelayCausesShare              DelayCausesShare      `json:"delay_causes_share"`
	DiversionAirports             []AirportCount        `json:"diversion_airports"`
	// Present only when the carrier query filter is omitted. Sorted by on_time_rate desc, then flights desc, then carrier asc.
	CarrierOnTime []CarrierOnTime `json:"carrier_on_time,omitzero"`
}

// RouteOutlook is the route outlook response. OutlookResult is embedded so
// an itinerary leg can return the same results without the request filters.
type RouteOutlook struct {
	Filters Filters `json:"filters"`
	OutlookResult
}

// OutlookResult is the outlook sample and estimates. Itinerary legs return this
// alone so origin, dest, carrier, and departure time are not repeated.
type OutlookResult struct {
	Analysis                        Analysis           `json:"analysis"`
	OnTimeProbability               *float64           `json:"on_time_probability"`
	DelayProbability                *float64           `json:"delay_probability"`
	CancellationProbability         *float64           `json:"cancellation_probability"`
	DiversionProbability            *float64           `json:"diversion_probability"`
	LikelyArrivalDelayMinutes       *float64           `json:"likely_arrival_delay_minutes"`
	MedianArrivalDelayMinutes       *float64           `json:"median_arrival_delay_minutes"`
	LikelyArrivalDelayWhenDelayed   *float64           `json:"likely_arrival_delay_when_delayed"`
	MedianArrivalDelayWhenDelayed   *float64           `json:"median_arrival_delay_when_delayed"`
	LikelyDepartureDelayMinutes     *float64           `json:"likely_departure_delay_minutes"`
	MedianDepartureDelayMinutes     *float64           `json:"median_departure_delay_minutes"`
	LikelyDepartureDelayWhenDelayed *float64           `json:"likely_departure_delay_when_delayed"`
	MedianDepartureDelayWhenDelayed *float64           `json:"median_departure_delay_when_delayed"`
	Connection                      ConnectionGuidance `json:"connection"`
	// Weather is route-level fair and adverse on-time rates, not this departure slot.
	Weather RouteOutlookWeather `json:"weather"`
}

// TravelWindowMonthBucket is one month-of-year on-time rate.
// confidence and reliability are seasonal only; they are not connection statuses.
// reliability is unknown unless confidence is high, so a thin bucket is not unreliable.
// Unreliable is a high-confidence on-time rate below 0.70. A month with no flights is omitted.
type TravelWindowMonthBucket struct {
	Month       int     `json:"month"`
	OnTimeRate  float64 `json:"on_time_rate"`
	Flights     int     `json:"flights"`
	Confidence  string  `json:"confidence" enums:"high,low,unknown"`
	Reliability string  `json:"reliability" enums:"reliable,typical,unreliable,unknown"`
}

// TravelWindowDayBucket is one weekday on-time rate.
// confidence and reliability are seasonal only; they are not connection statuses.
// reliability is unknown unless confidence is high, so a thin bucket is not unreliable.
// Unreliable is a high-confidence on-time rate below 0.70. A weekday with no flights is omitted.
type TravelWindowDayBucket struct {
	DayOfWeek   int     `json:"day_of_week"`
	OnTimeRate  float64 `json:"on_time_rate"`
	Flights     int     `json:"flights"`
	Confidence  string  `json:"confidence" enums:"high,low,unknown"`
	Reliability string  `json:"reliability" enums:"reliable,typical,unreliable,unknown"`
}

// TravelWindowHourBucket is one scheduled departure-hour on-time rate.
// confidence and reliability are seasonal only; they are not connection statuses.
// reliability is unknown unless confidence is high, so a thin bucket is not unreliable.
// Unreliable is a high-confidence on-time rate below 0.70. An hour with no flights is omitted.
type TravelWindowHourBucket struct {
	Hour        int     `json:"hour"`
	OnTimeRate  float64 `json:"on_time_rate"`
	Flights     int     `json:"flights"`
	Confidence  string  `json:"confidence" enums:"high,low,unknown"`
	Reliability string  `json:"reliability" enums:"reliable,typical,unreliable,unknown"`
}

// RouteTravelWindows is typical-year on-time performance for a route (and
// optional marketing carrier), pooled across the trailing analysis window.
type RouteTravelWindows struct {
	Filters     Filters                   `json:"filters"`
	Analysis    Analysis                  `json:"analysis"`
	ByMonth     []TravelWindowMonthBucket `json:"by_month"`
	ByDayOfWeek []TravelWindowDayBucket   `json:"by_day_of_week"`
	ByHour      []TravelWindowHourBucket  `json:"by_hour"`
	BestMonths  []TravelWindowMonthBucket `json:"best_months"`
	WorstMonths []TravelWindowMonthBucket `json:"worst_months"`
	BestDays    []TravelWindowDayBucket   `json:"best_days"`
	WorstDays   []TravelWindowDayBucket   `json:"worst_days"`
	BestHours   []TravelWindowHourBucket  `json:"best_hours"`
	WorstHours  []TravelWindowHourBucket  `json:"worst_hours"`
}

// RoundForResponse rounds rates and probabilities to two decimal places and
// minute values to the nearest minute.
func (s *RouteStats) RoundForResponse() {
	s.OnTimeRate = roundRate(s.OnTimeRate)
	s.DelayRate = roundRate(s.DelayRate)
	s.CancellationRate = roundRate(s.CancellationRate)
	s.DiversionRate = roundRate(s.DiversionRate)
	s.AvgArrivalDelayMinutes = math.Round(s.AvgArrivalDelayMinutes)
	s.MedianArrivalDelayMinutes = math.Round(s.MedianArrivalDelayMinutes)
	s.AvgArrivalDelayWhenDelayed = math.Round(s.AvgArrivalDelayWhenDelayed)
	s.MedianArrivalDelayWhenDelayed = math.Round(s.MedianArrivalDelayWhenDelayed)
	s.AvgDepartureDelayMinutes = math.Round(s.AvgDepartureDelayMinutes)
	s.MedianDepartureDelayMinutes = math.Round(s.MedianDepartureDelayMinutes)
	s.AvgDepartureDelayWhenDelayed = math.Round(s.AvgDepartureDelayWhenDelayed)
	s.DelayCausesAvgMinutes.Carrier = math.Round(s.DelayCausesAvgMinutes.Carrier)
	s.DelayCausesAvgMinutes.Weather = math.Round(s.DelayCausesAvgMinutes.Weather)
	s.DelayCausesAvgMinutes.NAS = math.Round(s.DelayCausesAvgMinutes.NAS)
	s.DelayCausesAvgMinutes.Security = math.Round(s.DelayCausesAvgMinutes.Security)
	s.DelayCausesAvgMinutes.LateAircraft = math.Round(s.DelayCausesAvgMinutes.LateAircraft)
	s.DelayCausesShare.round()

	for i := range s.CarrierOnTime {
		s.CarrierOnTime[i].round()
	}
}

// RoundForResponse rounds probabilities to two decimal places and minute
// values to the nearest minute. Null estimates stay null.
func (o *RouteOutlook) RoundForResponse() {
	o.OnTimeProbability = roundRatePtr(o.OnTimeProbability)
	o.DelayProbability = roundRatePtr(o.DelayProbability)
	o.CancellationProbability = roundRatePtr(o.CancellationProbability)
	o.DiversionProbability = roundRatePtr(o.DiversionProbability)
	o.LikelyArrivalDelayMinutes = roundMinutesPtr(o.LikelyArrivalDelayMinutes)
	o.MedianArrivalDelayMinutes = roundMinutesPtr(o.MedianArrivalDelayMinutes)
	o.LikelyArrivalDelayWhenDelayed = roundMinutesPtr(o.LikelyArrivalDelayWhenDelayed)
	o.MedianArrivalDelayWhenDelayed = roundMinutesPtr(o.MedianArrivalDelayWhenDelayed)
	o.LikelyDepartureDelayMinutes = roundMinutesPtr(o.LikelyDepartureDelayMinutes)
	o.MedianDepartureDelayMinutes = roundMinutesPtr(o.MedianDepartureDelayMinutes)
	o.LikelyDepartureDelayWhenDelayed = roundMinutesPtr(o.LikelyDepartureDelayWhenDelayed)
	o.MedianDepartureDelayWhenDelayed = roundMinutesPtr(o.MedianDepartureDelayWhenDelayed)
	o.Weather.RoundForResponse()
}

// RoundForResponse rounds on-time rates to two decimal places.
func (w *RouteTravelWindows) RoundForResponse() {
	for i := range w.ByMonth {
		w.ByMonth[i].OnTimeRate = roundRate(w.ByMonth[i].OnTimeRate)
	}

	for i := range w.ByDayOfWeek {
		w.ByDayOfWeek[i].OnTimeRate = roundRate(w.ByDayOfWeek[i].OnTimeRate)
	}

	for i := range w.ByHour {
		w.ByHour[i].OnTimeRate = roundRate(w.ByHour[i].OnTimeRate)
	}

	for i := range w.BestMonths {
		w.BestMonths[i].OnTimeRate = roundRate(w.BestMonths[i].OnTimeRate)
	}

	for i := range w.WorstMonths {
		w.WorstMonths[i].OnTimeRate = roundRate(w.WorstMonths[i].OnTimeRate)
	}

	for i := range w.BestDays {
		w.BestDays[i].OnTimeRate = roundRate(w.BestDays[i].OnTimeRate)
	}

	for i := range w.WorstDays {
		w.WorstDays[i].OnTimeRate = roundRate(w.WorstDays[i].OnTimeRate)
	}

	for i := range w.BestHours {
		w.BestHours[i].OnTimeRate = roundRate(w.BestHours[i].OnTimeRate)
	}

	for i := range w.WorstHours {
		w.WorstHours[i].OnTimeRate = roundRate(w.WorstHours[i].OnTimeRate)
	}
}

func (s *CarrierOnTime) round() {
	s.OnTimeRate = roundRate(s.OnTimeRate)
}

func (s *DelayCausesShare) round() {
	s.Carrier = roundRate(s.Carrier)
	s.Weather = roundRate(s.Weather)
	s.NAS = roundRate(s.NAS)
	s.Security = roundRate(s.Security)
	s.LateAircraft = roundRate(s.LateAircraft)
	s.Unattributed = roundRate(s.Unattributed)
}

func roundRate(v float64) float64 {
	return math.Round(v*100) / 100
}

func roundRatePtr(v *float64) *float64 {
	if v == nil {
		return nil
	}

	rounded := roundRate(*v)

	return &rounded
}

func roundMinutesPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}

	rounded := math.Round(*v)

	return &rounded
}
