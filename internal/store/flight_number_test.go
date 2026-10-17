package store

import "testing"

func TestNormalizeFlightNumber(t *testing.T) {
	tests := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		{raw: "123", want: "123", wantOK: true},
		{raw: "0123", want: "123", wantOK: true},
		{raw: " 0042 ", want: "42", wantOK: true},
		{raw: "0", want: "0", wantOK: true},
		{raw: ""},
		{raw: "   "},
		{raw: "UA123"},
		{raw: "-1"},
		{raw: "12.5"},
	}

	for _, tt := range tests {
		got, ok := NormalizeFlightNumber(tt.raw)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("NormalizeFlightNumber(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}
