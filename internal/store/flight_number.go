package store

import (
	"strconv"
	"strings"
)

const FlightNumberMaxLen = 4

type FlightNumberRoutesFilter struct {
	Carrier      string
	FlightNumber string
}

// NormalizeFlightNumber trims and drops leading zeros so "0123" and "123" match BTS.
func NormalizeFlightNumber(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return "", false
	}

	return strconv.Itoa(n), true
}

type RouteFlightDateRange struct {
	Start string
	End   string
}
