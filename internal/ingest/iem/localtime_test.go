package iem

import (
	"testing"
	"time"
)

func TestFlightLocalToUTCChicagoWinter(t *testing.T) {
	got, err := FlightLocalToUTC(testTzChicago, testFlightDateWinter, testHHMM0700)
	if err != nil {
		t.Fatalf("FlightLocalToUTC() error = %v", err)
	}

	want := time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.UTC().Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestFlightLocalToUTCSpringForward(t *testing.T) {
	tests := []struct {
		name string
		hhmm int
		want time.Time
	}{
		{
			name: "before spring forward",
			hhmm: 159,
			want: time.Date(2024, 3, 10, 7, 59, 0, 0, time.UTC),
		},
		{
			name: "after spring forward",
			hhmm: 300,
			want: time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FlightLocalToUTC(testTzChicago, "2024-03-10", tt.hhmm)
			if err != nil {
				t.Fatalf("FlightLocalToUTC() error = %v", err)
			}

			if !got.Equal(tt.want) {
				t.Fatalf("got %s, want %s", got.UTC().Format(time.RFC3339), tt.want.Format(time.RFC3339))
			}
		})
	}
}

func TestFlightLocalToUTCErrors(t *testing.T) {
	tests := []struct {
		name       string
		tzname     string
		flightDate string
		hhmm       int
	}{
		{name: "empty tzname", tzname: "", flightDate: testFlightDateWinter, hhmm: testHHMM0700},
		{name: "invalid tzname", tzname: "Not/AZone", flightDate: testFlightDateWinter, hhmm: testHHMM0700},
		{name: "invalid date", tzname: testTzChicago, flightDate: "2024-13-40", hhmm: testHHMM0700},
		{name: "hhmm too large", tzname: testTzChicago, flightDate: testFlightDateWinter, hhmm: 2400},
		{name: "invalid minutes", tzname: testTzChicago, flightDate: testFlightDateWinter, hhmm: 1261},
		{name: "negative hhmm", tzname: testTzChicago, flightDate: testFlightDateWinter, hhmm: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FlightLocalToUTC(tt.tzname, tt.flightDate, tt.hhmm); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
