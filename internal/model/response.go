package model

// Filters is what the caller asked for. Fields that do not apply to an endpoint are omitted.
type Filters struct {
	Origin               string `json:"origin,omitempty"`
	Dest                 string `json:"dest,omitempty"`
	Carrier              string `json:"carrier,omitempty"`
	FlightNumber         string `json:"flight_number,omitempty"`
	State                string `json:"state,omitempty"`
	StartDate            string `json:"start_date,omitempty"`
	EndDate              string `json:"end_date,omitempty"`
	DaysOfWeek           []int  `json:"days_of_week,omitempty"`
	DayOfWeek            int    `json:"day_of_week,omitempty"`
	DepTime              string `json:"dep_time,omitempty"`
	DepTimeWindowMinutes int    `json:"dep_time_window_minutes,omitempty"`
}

// Analysis is the date window actually used and the sample behind the result.
type Analysis struct {
	Start      string `json:"start"`
	End        string `json:"end"`
	SampleSize int    `json:"sample_size"`
	// SampleReason is sufficient, insufficient_sample, or empty_sample.
	// empty_sample is a seen route and carrier with no flights in this slot:
	// confidence is unknown and estimate fields are null.
	// Never-seen is 404 on route outlook, and a null outlook plus error on an itinerary leg.
	SampleReason string `json:"sample_reason,omitempty" enums:"sufficient,insufficient_sample,empty_sample"`
	// Confidence is high, low, or unknown for sufficient, insufficient_sample, and empty_sample.
	Confidence string `json:"confidence,omitempty" enums:"high,low,unknown"`
}
