package store

import "fmt"

// FlightYearMonthFromDate parses YYYY-MM-DD into year and month integers.
func FlightYearMonthFromDate(flightDate string) (year, month int, ok bool) {
	if len(flightDate) < 10 {
		return 0, 0, false
	}

	var day int
	if _, err := fmt.Sscanf(flightDate, "%d-%d-%d", &year, &month, &day); err != nil {
		return 0, 0, false
	}

	if month < 1 || month > 12 {
		return 0, 0, false
	}

	return year, month, true
}
