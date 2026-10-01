package model

import "testing"

func TestConnectionStatus(t *testing.T) {
	minutes := NewConnectionMinutes(60)

	tests := []struct {
		name    string
		layover int
		minutes *ConnectionMinutes
		want    string
	}{
		{name: "nil thresholds", layover: 120, want: ConnectionUnknown},
		{name: "loose boundary", layover: minutes.LooseMinutes, minutes: minutes, want: ConnectionLoose},
		{name: "just under loose", layover: minutes.LooseMinutes - 1, minutes: minutes, want: ConnectionOK},
		{name: "ok boundary", layover: minutes.RecommendedMinutes, minutes: minutes, want: ConnectionOK},
		{name: "just under recommended", layover: minutes.RecommendedMinutes - 1, minutes: minutes, want: ConnectionTight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConnectionStatus(tt.layover, tt.minutes); got != tt.want {
				t.Errorf("ConnectionStatus(%d) = %q, want %q", tt.layover, got, tt.want)
			}
		})
	}

	if minutes.LooseMinutes != minutes.RecommendedMinutes+ConnectionLooseSlackMinutes {
		t.Fatalf("loose = %d, want recommended + %d", minutes.LooseMinutes, ConnectionLooseSlackMinutes)
	}
}

func TestOutlookConfidence(t *testing.T) {
	tests := []struct {
		name         string
		sampleSize   int
		insufficient bool
		want         string
	}{
		{name: "empty sample", sampleSize: 0, want: ConfidenceUnknown},
		{name: "empty wins over flag", sampleSize: 0, insufficient: true, want: ConfidenceUnknown},
		{name: "thin sample", sampleSize: 4, insufficient: true, want: ConfidenceLow},
		{name: "usable sample", sampleSize: 40, want: ConfidenceHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OutlookConfidence(tt.sampleSize, tt.insufficient); got != tt.want {
				t.Errorf("OutlookConfidence() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecisionConfidence(t *testing.T) {
	if got := (*RouteOutlook)(nil).DecisionConfidence(); got != ConfidenceUnknown {
		t.Errorf("nil outlook confidence = %q", got)
	}

	explicit := &RouteOutlook{SampleSize: 0, Confidence: ConfidenceHigh}
	if got := explicit.DecisionConfidence(); got != ConfidenceHigh {
		t.Errorf("explicit confidence = %q, want high", got)
	}

	derived := &RouteOutlook{SampleSize: 3, InsufficientSample: true}
	if got := derived.DecisionConfidence(); got != ConfidenceLow {
		t.Errorf("derived confidence = %q, want low", got)
	}
}

func TestSeasonalDecision(t *testing.T) {
	tests := []struct {
		name            string
		flights         int
		rate            float64
		minSample       int
		wantConfidence  string
		wantReliability string
	}{
		{name: "no flights", minSample: 30, wantConfidence: ConfidenceUnknown, wantReliability: ReliabilityUnknown},
		{name: "below minimum", flights: 10, rate: 0.95, minSample: 30, wantConfidence: ConfidenceLow, wantReliability: ReliabilityUnknown},
		{name: "reliable boundary", flights: 30, rate: SeasonalReliableMinRate, minSample: 30, wantConfidence: ConfidenceHigh, wantReliability: ReliabilityReliable},
		{name: "typical just under reliable", flights: 30, rate: SeasonalReliableMinRate - 0.01, minSample: 30, wantConfidence: ConfidenceHigh, wantReliability: ReliabilityTypical},
		{name: "typical boundary", flights: 30, rate: SeasonalTypicalMinRate, minSample: 30, wantConfidence: ConfidenceHigh, wantReliability: ReliabilityTypical},
		{name: "unreliable", flights: 30, rate: SeasonalTypicalMinRate - 0.01, minSample: 30, wantConfidence: ConfidenceHigh, wantReliability: ReliabilityUnreliable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence, reliability := SeasonalDecision(tt.flights, tt.rate, tt.minSample)
			if confidence != tt.wantConfidence || reliability != tt.wantReliability {
				t.Errorf("SeasonalDecision() = %q/%q, want %q/%q", confidence, reliability, tt.wantConfidence, tt.wantReliability)
			}
		})
	}
}
