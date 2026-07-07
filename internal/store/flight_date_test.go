package store

import "testing"

func TestFlightYearMonthFromDate(t *testing.T) {
	year, month, ok := FlightYearMonthFromDate("2026-04-24")
	if !ok {
		t.Fatal("FlightYearMonthFromDate() ok = false, want true")
	}

	if year != 2026 || month != 4 {
		t.Errorf("got year=%d month=%d, want 2026 4", year, month)
	}
}
