package store

import (
	"errors"
	"strings"
)

const (
	// FlightNumberMaxLen is the digit count after leading zeros are removed.
	FlightNumberMaxLen = 4

	// FlightNumberRawMaxLen is the longest accepted raw flight number, including leading zeros.
	FlightNumberRawMaxLen = FlightNumberMaxLen * 2

	// FlightNumberPattern matches a raw flight number: optional leading zeros, then 1 to 4 digits, not all zeros.
	FlightNumberPattern = `^0*[1-9][0-9]{0,3}$`

	// MissingFlightNumber is the route-weather rollup key when the marketing flight number is blank.
	// Empty string is one primary-key value, so those flights share a single GROUP BY bucket.
	MissingFlightNumber = ""
)

// ErrInvalidFlightNumber is returned when a flight number is empty, not digits, all zeros, or longer than 4 digits after leading zeros are removed.
var ErrInvalidFlightNumber = errors.New("flight_number must be 1 to 4 digits")

type FlightNumberRoutesFilter struct {
	Carrier      string
	FlightNumber string
}

// NormalizeFlightNumber trims space and strips leading zeros so "0123" and "123" are the same flight.
// All zeros are invalid. A blank marketing number is missing, not flight 0.
func NormalizeFlightNumber(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > FlightNumberRawMaxLen {
		return "", ErrInvalidFlightNumber
	}

	for _, r := range raw {
		if r < '0' || r > '9' {
			return "", ErrInvalidFlightNumber
		}
	}

	raw = strings.TrimLeft(raw, "0")
	if raw == "" || len(raw) > FlightNumberMaxLen {
		return "", ErrInvalidFlightNumber
	}

	return raw, nil
}

// WeatherRollupFlightNumber is the route_weather_category_buckets key for one stored marketing flight number.
// Blank and any value NormalizeFlightNumber rejects share MissingFlightNumber so they compare equal.
func WeatherRollupFlightNumber(raw string) string {
	number, err := NormalizeFlightNumber(raw)
	if err != nil {
		return MissingFlightNumber
	}

	return number
}
