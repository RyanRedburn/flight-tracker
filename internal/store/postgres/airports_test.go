package postgres

import "testing"

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
