package postgres

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestNormalizeAirportQuery(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "  Chicago  ", want: "Chicago"},
		{in: "new   york", want: "new york"},
		{in: " \t ", want: ""},
	}

	for _, tt := range tests {
		if got := normalizeAirportQuery(tt.in); got != tt.want {
			t.Errorf("normalizeAirportQuery(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAirportSearchLimit(t *testing.T) {
	if got := airportSearchLimit(0); got != store.AirportSearchDefaultLimit {
		t.Errorf("default = %d", got)
	}

	if got := airportSearchLimit(-4); got != store.AirportSearchDefaultLimit {
		t.Errorf("negative = %d", got)
	}

	if got := airportSearchLimit(3); got != 3 {
		t.Errorf("within max = %d", got)
	}

	if got := airportSearchLimit(store.AirportSearchMaxLimit + 10); got != store.AirportSearchMaxLimit {
		t.Errorf("above max = %d", got)
	}
}

func TestAirportSearchPatterns(t *testing.T) {
	contains, prefix := airportSearchPatterns(`100%_`)
	if contains != `%100\%\_%` || prefix != `100\%\_%` {
		t.Fatalf("patterns = %q %q", contains, prefix)
	}

	contains, prefix = airportSearchPatterns(`a\b`)
	if contains != `%a\\b%` || prefix != `a\\b%` {
		t.Fatalf("backslash patterns = %q %q", contains, prefix)
	}
}

func TestLooksLikeIATA(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{in: testAirportORD, want: true},
		{in: "ord", want: true},
		{in: "OR", want: false},
		{in: "ORDX", want: false},
		{in: "OR1", want: false},
		{in: "New York", want: false},
	}

	for _, tt := range tests {
		if got := looksLikeIATA(tt.in); got != tt.want {
			t.Errorf("looksLikeIATA(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
