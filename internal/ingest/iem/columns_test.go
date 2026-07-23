package iem

import "testing"

func TestCSVHeaderToColumn(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{header: "station", want: "station"},
		{header: "  tmpf  ", want: "tmpf"},
		{header: "", want: ""},
	}

	for _, tt := range tests {
		if got := csvHeaderToColumn(tt.header); got != tt.want {
			t.Errorf("csvHeaderToColumn(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
