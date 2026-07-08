package model

type DelayCausesAvgMinutes struct {
	Carrier      float64 `json:"carrier"`
	Weather      float64 `json:"weather"`
	NAS          float64 `json:"nas"`
	Security     float64 `json:"security"`
	LateAircraft float64 `json:"late_aircraft"`
}

type AirportCount struct {
	Airport string `json:"airport"`
	Count   int    `json:"count"`
}

type CancellationCodeCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type RouteStatsFilters struct {
	Carrier      string `json:"carrier"`
	FlightNumber string `json:"flight_number"`
	DaysOfWeek   []int  `json:"days_of_week"`
}

type RouteStats struct {
	Origin                        string                  `json:"origin"`
	Dest                          string                  `json:"dest"`
	StartDate                     string                  `json:"start_date"`
	EndDate                       string                  `json:"end_date"`
	Filters                       RouteStatsFilters       `json:"filters"`
	Flights                       int                     `json:"flights"`
	OnTime                        int                     `json:"on_time"`
	Delayed                       int                     `json:"delayed"`
	Cancelled                     int                     `json:"cancelled"`
	Diverted                      int                     `json:"diverted"`
	OnTimeRate                    float64                 `json:"on_time_rate"`
	DelayRate                     float64                 `json:"delay_rate"`
	CancellationRate              float64                 `json:"cancellation_rate"`
	DiversionRate                 float64                 `json:"diversion_rate"`
	AvgArrivalDelayMinutes        float64                 `json:"avg_arrival_delay_minutes"`
	MedianArrivalDelayMinutes     float64                 `json:"median_arrival_delay_minutes"`
	AvgArrivalDelayWhenDelayed    float64                 `json:"avg_arrival_delay_when_delayed"`
	MedianArrivalDelayWhenDelayed float64                 `json:"median_arrival_delay_when_delayed"`
	AvgDepartureDelayMinutes      float64                 `json:"avg_departure_delay_minutes"`
	AvgDepartureDelayWhenDelayed  float64                 `json:"avg_departure_delay_when_delayed"`
	DelayCausesAvgMinutes         DelayCausesAvgMinutes   `json:"delay_causes_avg_minutes"`
	DiversionAirports             []AirportCount          `json:"diversion_airports"`
	CancellationCodes             []CancellationCodeCount `json:"cancellation_codes"`
}

type RouteOutlook struct {
	Origin                        string  `json:"origin"`
	Dest                          string  `json:"dest"`
	Carrier                       string  `json:"carrier"`
	DayOfWeek                     int     `json:"day_of_week"`
	DepTime                       string  `json:"dep_time"`
	DepTimeWindowMinutes          int     `json:"dep_time_window_minutes"`
	AnalysisStart                 string  `json:"analysis_start"`
	AnalysisEnd                   string  `json:"analysis_end"`
	SampleSize                    int     `json:"sample_size"`
	InsufficientSample            bool    `json:"insufficient_sample"`
	OnTimeProbability             float64 `json:"on_time_probability"`
	DelayProbability              float64 `json:"delay_probability"`
	CancellationProbability       float64 `json:"cancellation_probability"`
	DiversionProbability          float64 `json:"diversion_probability"`
	LikelyArrivalDelayMinutes     float64 `json:"likely_arrival_delay_minutes"`
	MedianArrivalDelayMinutes     float64 `json:"median_arrival_delay_minutes"`
	LikelyArrivalDelayWhenDelayed float64 `json:"likely_arrival_delay_when_delayed"`
	MedianArrivalDelayWhenDelayed float64 `json:"median_arrival_delay_when_delayed"`
	LikelyDepartureDelayMinutes   float64 `json:"likely_departure_delay_minutes"`
}
