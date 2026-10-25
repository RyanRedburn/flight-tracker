package model

// Filters is the request. Fields that do not apply, or that the caller omitted, are left out.
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
	// empty_sample means the slot has no flights: confidence is unknown and estimates are null.
	// sufficient covers medium and high. insufficient_sample is low.
	SampleReason string `json:"sample_reason,omitempty" enums:"sufficient,insufficient_sample,empty_sample"`
	Confidence   string `json:"confidence,omitempty" enums:"high,medium,low,unknown"`
	// Days of history used: 365, or 730 when a thin default window was widened.
	LookbackDays int `json:"lookback_days,omitempty"`
}
