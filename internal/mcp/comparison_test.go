package mcp

import "testing"

func TestSummaryWording(t *testing.T) {
	items := []scoredItem{
		{label: "UA", percent: 54, rate: 0.54, sampleSize: 100},
		{label: "AA", percent: 55, rate: 0.55, sampleSize: 100},
		{label: "DL", percent: 50, rate: 0.50, sampleSize: 100},
		{label: "WN", percent: 52, rate: 0.52, sampleSize: 100},
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "one trails", got: appendTrailing("Lead.", items[:1]), want: "Lead. 1 other also trails (54%)."},
		{name: "two trail", got: appendTrailing("Lead.", items[:2]), want: "Lead. 2 others also trail (54–55%)."},
		{name: "one tied", got: appendTied("Lead.", items[:1]), want: "Lead. 1 other is also too close to call (54%)."},
		{name: "two tied", got: appendTied("Lead.", items[:2]), want: "Lead. 2 others are also too close to call (54–55%)."},
		{name: "three named by rate", got: tooCloseLabel(items[:3], nounCarrier), want: "AA, UA and DL"},
		{name: "four counted", got: tooCloseLabel(items, nounCarrier), want: "All 4 carriers"},
		{name: "equal flights", got: flightsPhrase([]int{2000, 2000, 2000}), want: "2000 flights each"},
		{name: "one flight each", got: flightsPhrase([]int{1, 1}), want: "1 flight each"},
		{name: "uneven flights", got: flightsPhrase([]int{100, 8, 4}), want: "100, 8 and 4 flights"},
		{name: "single flight", got: flightsPhrase([]int{1}), want: "1 flight"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}
