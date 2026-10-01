package model

const (
	// Connection status compares a same-airport layover with the inbound outlook thresholds.
	// Travel-window buckets do not use these values.
	ConnectionLoose   = "loose"
	ConnectionOK      = "ok"
	ConnectionTight   = "tight"
	ConnectionUnknown = "unknown"

	// Confidence is how much history supports a decision.
	// Outlook, a classified itinerary connection, and travel-window buckets share these words.
	ConfidenceHigh    = "high"
	ConfidenceLow     = "low"
	ConfidenceUnknown = "unknown"

	// Seasonal reliability is typical-year on-time performance for a travel-window bucket.
	// It is not a connection status.
	// Unreliable is a high-confidence verdict. A thin bucket stays reliability unknown.
	ReliabilityReliable   = "reliable"
	ReliabilityTypical    = "typical"
	ReliabilityUnreliable = "unreliable"
	ReliabilityUnknown    = "unknown"

	// SampleReason names the outlook slot behind confidence and insufficient_sample.
	// It is the machine-readable sample state for a 200 response. It is not a
	// second sparse boolean, and it is not a connection status.
	// A route and carrier that have never been seen are not a sample reason:
	// route outlook is 404, and an itinerary leg has a null outlook plus error.
	SampleReasonSufficient         = "sufficient"
	SampleReasonInsufficientSample = "insufficient_sample"
	SampleReasonEmptySample        = "empty_sample"
)

const (
	// ConnectionLooseSlackMinutes is the extra slack, above recommended_minutes,
	// required before a connection is loose instead of ok.
	ConnectionLooseSlackMinutes = 30

	// SeasonalReliableMinRate and SeasonalTypicalMinRate apply to a bucket's
	// on-time rate only when that bucket's confidence is high.
	SeasonalReliableMinRate = 0.80
	SeasonalTypicalMinRate  = 0.70
)

// ConnectionMinutes is the layover threshold for one same-airport connection type.
// recommended_minutes is the smallest layover that is ok.
// loose_minutes is 30 minutes higher; a layover that long is loose.
// A shorter layover than recommended_minutes is tight.
type ConnectionMinutes struct {
	RecommendedMinutes int `json:"recommended_minutes"`
	LooseMinutes       int `json:"loose_minutes"`
}

// ConnectionGuidance is per connection type for one inbound outlook.
// A null bucket cannot be estimated, so an itinerary status for that type is unknown.
type ConnectionGuidance struct {
	DomesticToDomestic           *ConnectionMinutes `json:"domestic_to_domestic"`
	DomesticToInternational      *ConnectionMinutes `json:"domestic_to_international"`
	InternationalToDomestic      *ConnectionMinutes `json:"international_to_domestic"`
	InternationalToInternational *ConnectionMinutes `json:"international_to_international"`
}

// NewConnectionMinutes returns thresholds whose loose boundary is recommended plus 30 minutes.
func NewConnectionMinutes(recommended int) *ConnectionMinutes {
	return &ConnectionMinutes{
		RecommendedMinutes: recommended,
		LooseMinutes:       recommended + ConnectionLooseSlackMinutes,
	}
}

// ConnectionGuidanceFromRecommended builds all four connection types from recommended minutes.
func ConnectionGuidanceFromRecommended(domesticToDomestic, domesticToInternational, internationalToDomestic, internationalToInternational int) ConnectionGuidance {
	return ConnectionGuidance{
		DomesticToDomestic:           NewConnectionMinutes(domesticToDomestic),
		DomesticToInternational:      NewConnectionMinutes(domesticToInternational),
		InternationalToDomestic:      NewConnectionMinutes(internationalToDomestic),
		InternationalToInternational: NewConnectionMinutes(internationalToInternational),
	}
}

// ForType returns the threshold for a connection type, or nil when that type is unset or unknown.
func (g ConnectionGuidance) ForType(connectionType string) *ConnectionMinutes {
	switch connectionType {
	case ConnectionDomesticToDomestic:
		return g.DomesticToDomestic
	case ConnectionDomesticToInternational:
		return g.DomesticToInternational
	case ConnectionInternationalToDomestic:
		return g.InternationalToDomestic
	case ConnectionInternationalToInternational:
		return g.InternationalToInternational
	default:
		return nil
	}
}

// ConnectionStatus classifies a layover against published thresholds.
// Nil thresholds are unknown. The comparison uses those minute fields, not a second formula.
func ConnectionStatus(layover int, minutes *ConnectionMinutes) string {
	if minutes == nil {
		return ConnectionUnknown
	}

	if layover >= minutes.LooseMinutes {
		return ConnectionLoose
	}

	if layover >= minutes.RecommendedMinutes {
		return ConnectionOK
	}

	return ConnectionTight
}

// OutlookConfidence maps an outlook sample onto high, low, or unknown.
// insufficientSample is the existing thin-history flag (some rows, below the minimum).
// An empty sample is unknown even when that flag is false.
func OutlookConfidence(sampleSize int, insufficientSample bool) string {
	if sampleSize <= 0 {
		return ConfidenceUnknown
	}

	if insufficientSample {
		return ConfidenceLow
	}

	return ConfidenceHigh
}

// ApplyOutlookSample sets insufficient_sample, sample_reason, and confidence from the slot size.
// sufficient: sample meets minSample. The bool is false and confidence is high.
// insufficient_sample: some flights fall short of minSample. The bool is true and confidence is low.
// Estimates stay, and connection thresholds are cleared.
// empty_sample: no flights in this weekday and departure window. The bool stays false,
// confidence is unknown, and probability, delay, and connection fields are cleared
// so a zero is not a forecast.
// minSample <= 0 treats every non-empty slot as sufficient.
func (o *RouteOutlook) ApplyOutlookSample(minSample int) {
	if o == nil {
		return
	}

	insufficient := o.SampleSize > 0 && minSample > 0 && o.SampleSize < minSample
	o.InsufficientSample = insufficient
	o.Confidence = OutlookConfidence(o.SampleSize, insufficient)

	switch o.Confidence {
	case ConfidenceHigh:
		o.SampleReason = SampleReasonSufficient
	case ConfidenceLow:
		o.SampleReason = SampleReasonInsufficientSample
		o.Connection = ConnectionGuidance{}
	default:
		if o.SampleSize < 0 {
			o.SampleSize = 0
		}

		o.SampleReason = SampleReasonEmptySample
		o.clearEstimates()
	}
}

func (o *RouteOutlook) clearEstimates() {
	o.OnTimeProbability = nil
	o.DelayProbability = nil
	o.CancellationProbability = nil
	o.DiversionProbability = nil
	o.LikelyArrivalDelayMinutes = nil
	o.MedianArrivalDelayMinutes = nil
	o.LikelyArrivalDelayWhenDelayed = nil
	o.MedianArrivalDelayWhenDelayed = nil
	o.LikelyDepartureDelayMinutes = nil
	o.Connection = ConnectionGuidance{}
}

// DecisionConfidence is the confidence value an itinerary copies from this outlook.
// A recognized label is kept. Anything else is derived from the sample fields.
func (o *RouteOutlook) DecisionConfidence() string {
	if o == nil {
		return ConfidenceUnknown
	}

	switch o.Confidence {
	case ConfidenceHigh, ConfidenceLow, ConfidenceUnknown:
		return o.Confidence
	default:
		return OutlookConfidence(o.SampleSize, o.InsufficientSample)
	}
}

// SeasonalConfidence is high when flights meet minSample, low when some flights fall short, and unknown at zero.
func SeasonalConfidence(flights, minSample int) string {
	if flights <= 0 {
		return ConfidenceUnknown
	}

	if flights < minSample {
		return ConfidenceLow
	}

	return ConfidenceHigh
}

// SeasonalReliability classifies a rounded on-time rate. It is unknown unless confidence is high.
func SeasonalReliability(onTimeRate float64, confidence string) string {
	if confidence != ConfidenceHigh {
		return ReliabilityUnknown
	}

	if onTimeRate >= SeasonalReliableMinRate {
		return ReliabilityReliable
	}

	if onTimeRate >= SeasonalTypicalMinRate {
		return ReliabilityTypical
	}

	return ReliabilityUnreliable
}

// SeasonalDecision is the confidence and reliability pair for one travel-window bucket.
func SeasonalDecision(flights int, onTimeRate float64, minSample int) (confidence, reliability string) {
	confidence = SeasonalConfidence(flights, minSample)
	reliability = SeasonalReliability(onTimeRate, confidence)

	return confidence, reliability
}
