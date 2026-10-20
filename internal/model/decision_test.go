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

func TestMinConfidence(t *testing.T) {
	if got := MinConfidence(); got != ConfidenceUnknown {
		t.Fatalf("MinConfidence() = %q", got)
	}

	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "high alone", values: []string{ConfidenceHigh}, want: ConfidenceHigh},
		{name: "low weakens high", values: []string{ConfidenceHigh, ConfidenceLow}, want: ConfidenceLow},
		{name: "unknown weakest", values: []string{ConfidenceLow, ConfidenceUnknown, ConfidenceHigh}, want: ConfidenceUnknown},
		{name: "blank is weaker than high", values: []string{ConfidenceHigh, ""}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MinConfidence(tt.values...); got != tt.want {
				t.Fatalf("MinConfidence() = %q, want %q", got, tt.want)
			}
		})
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

func TestApplyOutlookSample(t *testing.T) {
	(*RouteOutlook)(nil).ApplyOutlookSample(10)

	zero := 0.0
	planted := ConnectionGuidanceFromRecommended(45, 60, 90, 120)

	tests := []struct {
		name               string
		sampleSize         int
		minSample          int
		wantReason         string
		wantConfidence     string
		wantConnectionKept bool
	}{
		{name: "empty slot", minSample: 10, wantReason: SampleReasonEmptySample, wantConfidence: ConfidenceUnknown},
		{name: "negative size", sampleSize: -3, minSample: 10, wantReason: SampleReasonEmptySample, wantConfidence: ConfidenceUnknown},
		{name: "thin sample", sampleSize: 4, minSample: 10, wantReason: SampleReasonInsufficientSample, wantConfidence: ConfidenceLow},
		{name: "one below minimum", sampleSize: 9, minSample: 10, wantReason: SampleReasonInsufficientSample, wantConfidence: ConfidenceLow},
		{name: "meets minimum", sampleSize: 10, minSample: 10, wantReason: SampleReasonSufficient, wantConfidence: ConfidenceHigh, wantConnectionKept: true},
		{name: "solid sample", sampleSize: 40, minSample: 10, wantReason: SampleReasonSufficient, wantConfidence: ConfidenceHigh, wantConnectionKept: true},
		{name: "no minimum treats non-empty as sufficient", sampleSize: 1, wantReason: SampleReasonSufficient, wantConfidence: ConfidenceHigh, wantConnectionKept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &RouteOutlook{
				OutlookResult: OutlookResult{
					Analysis:                  Analysis{SampleSize: tt.sampleSize},
					OnTimeProbability:         &zero,
					LikelyArrivalDelayMinutes: &zero,
					Connection:                planted,
				},
			}
			out.ApplyOutlookSample(tt.minSample)

			if out.Analysis.SampleReason != tt.wantReason || out.Analysis.Confidence != tt.wantConfidence {
				t.Fatalf("sample = reason %q confidence %q", out.Analysis.SampleReason, out.Analysis.Confidence)
			}

			if tt.sampleSize < 0 && out.Analysis.SampleSize != 0 {
				t.Fatalf("sample_size = %d, want 0", out.Analysis.SampleSize)
			}

			if tt.wantReason == SampleReasonEmptySample {
				if out.OnTimeProbability != nil || out.LikelyArrivalDelayMinutes != nil || out.Connection.DomesticToDomestic != nil {
					t.Fatalf("empty estimates = prob %v delay %v connection %+v", out.OnTimeProbability, out.LikelyArrivalDelayMinutes, out.Connection.DomesticToDomestic)
				}

				return
			}

			if out.OnTimeProbability == nil || *out.OnTimeProbability != 0 {
				t.Fatalf("on_time_probability = %v, want 0", out.OnTimeProbability)
			}

			kept := out.Connection.DomesticToDomestic != nil
			if kept != tt.wantConnectionKept {
				t.Fatalf("connection kept = %v, want %v", kept, tt.wantConnectionKept)
			}
		})
	}
}

func TestBlockingSampleReason(t *testing.T) {
	if got := (*RouteOutlook)(nil).BlockingSampleReason(); got != "" {
		t.Fatalf("nil blocking reason = %q", got)
	}

	tests := []struct {
		name    string
		outlook RouteOutlook
		want    string
	}{
		{name: "explicit sufficient", outlook: sampleOutlook(40, SampleReasonSufficient, ConfidenceHigh), want: ""},
		{name: "explicit empty", outlook: sampleOutlook(0, SampleReasonEmptySample, ""), want: SampleReasonEmptySample},
		{name: "explicit thin", outlook: sampleOutlook(4, SampleReasonInsufficientSample, ConfidenceLow), want: SampleReasonInsufficientSample},
		{name: "explicit empty wins over thresholds", outlook: withConnection(sampleOutlook(0, SampleReasonEmptySample, ConfidenceHigh), ConnectionGuidanceFromRecommended(45, 60, 120, 120)), want: SampleReasonEmptySample},
		{name: "derived thin", outlook: sampleOutlook(4, "", ConfidenceLow), want: SampleReasonInsufficientSample},
		{name: "derived low confidence", outlook: sampleOutlook(40, "", ConfidenceLow), want: SampleReasonInsufficientSample},
		{name: "derived empty", outlook: sampleOutlook(0, "", ConfidenceUnknown), want: SampleReasonEmptySample},
		{name: "derived sufficient from size", outlook: sampleOutlook(40, "", ConfidenceHigh), want: ""},
		{name: "explicit high with empty size stays open", outlook: sampleOutlook(0, "", ConfidenceHigh), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.outlook.BlockingSampleReason(); got != tt.want {
				t.Errorf("BlockingSampleReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecisionConfidence(t *testing.T) {
	if got := (*RouteOutlook)(nil).DecisionConfidence(); got != ConfidenceUnknown {
		t.Errorf("nil outlook confidence = %q", got)
	}

	explicit := sampleOutlook(0, "", ConfidenceHigh)
	if got := explicit.DecisionConfidence(); got != ConfidenceHigh {
		t.Errorf("explicit confidence = %q, want high", got)
	}

	derived := sampleOutlook(3, SampleReasonInsufficientSample, "")
	if got := derived.DecisionConfidence(); got != ConfidenceLow {
		t.Errorf("derived confidence = %q, want low", got)
	}
}

func sampleOutlook(size int, reason, confidence string) RouteOutlook {
	return RouteOutlook{
		OutlookResult: OutlookResult{
			Analysis: Analysis{
				SampleSize:   size,
				SampleReason: reason,
				Confidence:   confidence,
			},
		},
	}
}

func withConnection(outlook RouteOutlook, connection ConnectionGuidance) RouteOutlook {
	outlook.Connection = connection
	return outlook
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
