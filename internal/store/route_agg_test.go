package store

import "testing"

const (
	testFlightDate20260401 = "2026-04-01"
	testAirportLAX         = "LAX"
)

func TestHHMMToMinutes(t *testing.T) {
	tests := []struct {
		raw    string
		want   int
		wantOK bool
	}{
		{"0700", 420, true},
		{"700", 420, true},
		{"2350", 1430, true},
		{"2400", 0, false},
		{"ab", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := HHMMToMinutes(tt.raw)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("HHMMToMinutes(%q) = (%d,%v), want (%d,%v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}
