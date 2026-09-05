package model

import "math"

type CarrierStatsFilters struct {
	State string `json:"state"`
}

type CarrierRouteStat struct {
	Origin           string  `json:"origin"`
	Dest             string  `json:"dest"`
	Flights          int     `json:"flights"`
	OnTime           int     `json:"on_time"`
	Delayed          int     `json:"delayed"`
	Cancelled        int     `json:"cancelled"`
	Diverted         int     `json:"diverted"`
	OnTimeRate       float64 `json:"on_time_rate"`
	DelayRate        float64 `json:"delay_rate"`
	CancellationRate float64 `json:"cancellation_rate"`
	DiversionRate    float64 `json:"diversion_rate"`
}

type CarrierAirportStat struct {
	Airport          string  `json:"airport"`
	Flights          int     `json:"flights"`
	OnTime           int     `json:"on_time"`
	Delayed          int     `json:"delayed"`
	Cancelled        int     `json:"cancelled"`
	Diverted         int     `json:"diverted"`
	OnTimeRate       float64 `json:"on_time_rate"`
	DelayRate        float64 `json:"delay_rate"`
	CancellationRate float64 `json:"cancellation_rate"`
	DiversionRate    float64 `json:"diversion_rate"`
}

type CarrierStats struct {
	Carrier                       string                `json:"carrier"`
	StartDate                     string                `json:"start_date"`
	EndDate                       string                `json:"end_date"`
	Filters                       CarrierStatsFilters   `json:"filters"`
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
	AvgDepartureDelayWhenDelayed  float64               `json:"avg_departure_delay_when_delayed"`
	DelayCausesAvgMinutes         DelayCausesAvgMinutes `json:"delay_causes_avg_minutes"`
	DelayCausesShare              DelayCausesShare      `json:"delay_causes_share"`
	BestRoutes                    []CarrierRouteStat    `json:"best_routes"`
	WorstRoutes                   []CarrierRouteStat    `json:"worst_routes"`
	BestAirports                  []CarrierAirportStat  `json:"best_airports"`
	WorstAirports                 []CarrierAirportStat  `json:"worst_airports"`
}

// RoundForResponse rounds rates and shares to two decimal places and minute
// values to the nearest minute.
func (s *CarrierStats) RoundForResponse() {
	s.OnTimeRate = roundRate(s.OnTimeRate)
	s.DelayRate = roundRate(s.DelayRate)
	s.CancellationRate = roundRate(s.CancellationRate)
	s.DiversionRate = roundRate(s.DiversionRate)
	s.AvgArrivalDelayMinutes = math.Round(s.AvgArrivalDelayMinutes)
	s.MedianArrivalDelayMinutes = math.Round(s.MedianArrivalDelayMinutes)
	s.AvgArrivalDelayWhenDelayed = math.Round(s.AvgArrivalDelayWhenDelayed)
	s.MedianArrivalDelayWhenDelayed = math.Round(s.MedianArrivalDelayWhenDelayed)
	s.AvgDepartureDelayMinutes = math.Round(s.AvgDepartureDelayMinutes)
	s.AvgDepartureDelayWhenDelayed = math.Round(s.AvgDepartureDelayWhenDelayed)
	s.DelayCausesAvgMinutes.Carrier = math.Round(s.DelayCausesAvgMinutes.Carrier)
	s.DelayCausesAvgMinutes.Weather = math.Round(s.DelayCausesAvgMinutes.Weather)
	s.DelayCausesAvgMinutes.NAS = math.Round(s.DelayCausesAvgMinutes.NAS)
	s.DelayCausesAvgMinutes.Security = math.Round(s.DelayCausesAvgMinutes.Security)
	s.DelayCausesAvgMinutes.LateAircraft = math.Round(s.DelayCausesAvgMinutes.LateAircraft)
	s.DelayCausesShare.round()

	for i := range s.BestRoutes {
		s.BestRoutes[i].round()
	}

	for i := range s.WorstRoutes {
		s.WorstRoutes[i].round()
	}

	for i := range s.BestAirports {
		s.BestAirports[i].round()
	}

	for i := range s.WorstAirports {
		s.WorstAirports[i].round()
	}
}

func (s *CarrierRouteStat) round() {
	s.OnTimeRate = roundRate(s.OnTimeRate)
	s.DelayRate = roundRate(s.DelayRate)
	s.CancellationRate = roundRate(s.CancellationRate)
	s.DiversionRate = roundRate(s.DiversionRate)
}

func (s *CarrierAirportStat) round() {
	s.OnTimeRate = roundRate(s.OnTimeRate)
	s.DelayRate = roundRate(s.DelayRate)
	s.CancellationRate = roundRate(s.CancellationRate)
	s.DiversionRate = roundRate(s.DiversionRate)
}
