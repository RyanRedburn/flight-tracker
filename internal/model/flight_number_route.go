package model

// FlightNumberRoute is one origin–destination pair a marketing flight number has
// operated recently, with how often and its typical scheduled departure time.
type FlightNumberRoute struct {
	Origin       string `json:"origin"`
	Dest         string `json:"dest"`
	Flights      int    `json:"flights"`
	UsualDepTime string `json:"usual_dep_time"`
}
