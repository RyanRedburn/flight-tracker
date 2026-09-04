package iem

import (
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
)

// FlightLocalToUTC interprets a BTS civil flight date plus hhmm in tzname and returns UTC.
//
// Postgres equivalent (tzname from airport_weather_stations.tzname / weather_stations.tzname):
//
//	((flight_date + make_time((hhmm / 100), (hhmm % 100), 0)) AT TIME ZONE tzname)
func FlightLocalToUTC(tzname, flightDate string, hhmm int) (time.Time, error) {
	tzname = strings.TrimSpace(tzname)
	if tzname == "" {
		return time.Time{}, errors.New("tzname required")
	}

	loc, err := time.LoadLocation(tzname)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid tzname %q: %w", tzname, err)
	}

	flightDate = strings.TrimSpace(flightDate)

	day, err := time.ParseInLocation("2006-01-02", flightDate, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid flight date %q: %w", flightDate, err)
	}

	hour, minute, err := parseHHMM(hhmm)
	if err != nil {
		return time.Time{}, err
	}

	local := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc)

	return local.UTC(), nil
}

func parseHHMM(hhmm int) (hour, minute int, err error) {
	hour = hhmm / 100
	minute = hhmm % 100

	if hhmm < 0 || hhmm > 2359 || hour > 23 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid hhmm %d", hhmm)
	}

	return hour, minute, nil
}
