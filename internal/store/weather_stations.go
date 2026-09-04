package store

import "time"

// AirportWeatherStation is one BTS airport code mapped to an IEM ASOS site.
type AirportWeatherStation struct {
	AirportCode string
	IEMSID      string
	TzName      string
	Matched     bool
	UpdatedAt   time.Time
}

// AirportIdentifiers are OurAirports codes used to resolve an IEM ASOS sid.
type AirportIdentifiers struct {
	IATACode  string
	LocalCode string
	ICAOCode  string
	Ident     string
}
