package model

import "math"

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
	ConfidenceMedium  = "medium"
	ConfidenceLow     = "low"
	ConfidenceUnknown = "unknown"

	// ComparisonRanked means at least one pair differs at 95%.
	// ComparisonTooCloseToCall means every compared pair is inside the margin of error.
	ComparisonRanked         = "ranked"
	ComparisonTooCloseToCall = "too_close_to_call"

	// proportionZCritical is the two-sided 95% critical value for a pooled two-proportion z-test.
	proportionZCritical = 1.96

	// Seasonal reliability is typical-year on-time performance for a travel-window bucket.
	// It is not a connection status.
	// Unreliable is a medium- or high-confidence verdict. Low confidence stays reliability unknown.
	ReliabilityReliable   = "reliable"
	ReliabilityTypical    = "typical"
	ReliabilityUnreliable = "unreliable"
	ReliabilityUnknown    = "unknown"

	// SampleReason names the outlook slot behind confidence.
	SampleReasonSufficient         = "sufficient"
	SampleReasonInsufficientSample = "insufficient_sample"
	SampleReasonEmptySample        = "empty_sample"

	// ConnectionReason explains an itinerary status, including unknown.
	// empty_sample and insufficient_sample are the inbound outlook sample_reason.
	// missing_outlook is a null inbound outlook (route outlook 404 / leg error),
	// which is not empty_sample.
	ConnectionReasonEvaluated        = "evaluated"
	ConnectionReasonOvernight        = "overnight"
	ConnectionReasonMultiAirport     = "multi_airport"
	ConnectionReasonMissingCountry   = "missing_country"
	ConnectionReasonMissingOutlook   = "missing_outlook"
	ConnectionReasonMissingThreshold = "missing_threshold"
	ConnectionReasonNegativeLayover  = "negative_layover"
	ConnectionReasonInvalidSchedule  = "invalid_schedule"

	// MCTSourceAirport means the stored airport minimum connection time was used.
	// MCTSourceDefault means that bucket was missing, so the static floor was used.
	MCTSourceAirport = "airport"
	MCTSourceDefault = "default"
)

const (
	// ConnectionLooseSlackMinutes is the extra slack, above recommended_minutes,
	// required before a connection is loose instead of ok.
	ConnectionLooseSlackMinutes = 30

	// OutlookLowSampleBelow and OutlookHighSampleAt are the outlook and weather bands.
	// Below 30 is low, 30–49 is medium, and 50 or more is high.
	OutlookLowSampleBelow = 30
	OutlookHighSampleAt   = 50

	// HistoryLowSampleBelow and HistoryHighSampleAt are the carrier and month bands.
	// Below 100 is low, 100–299 is medium, and 300 or more is high.
	HistoryLowSampleBelow = 100
	HistoryHighSampleAt   = 300

	// SeasonalReliableMinRate and SeasonalTypicalMinRate apply when confidence is medium or high.
	SeasonalReliableMinRate = 0.80
	SeasonalTypicalMinRate  = 0.70
)

// ConnectionMinutes is the layover threshold for one same-airport connection type.
// recommended_minutes is arrival-delay p90 plus the destination airport MCT, or 45/60/120/120 when that minute is missing.
// It is the smallest layover that is ok. loose_minutes is 30 minutes higher. A shorter layover is tight.
// These minutes are not an official airline MCT.
type ConnectionMinutes struct {
	RecommendedMinutes int `json:"recommended_minutes"`
	LooseMinutes       int `json:"loose_minutes"`
	// MCTSource is airport when this bucket used the stored airport minute, or default when that minute was missing.
	MCTSource string `json:"mct_source" enums:"airport,default"`
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

// ProportionsDiffer reports whether two rates differ at 95% under a pooled two-proportion z-test.
// A gap at or inside the margin of error does not. A non-positive sample does not differ.
func ProportionsDiffer(rate1 float64, flights1 int, rate2 float64, flights2 int) bool {
	if flights1 <= 0 || flights2 <= 0 {
		return false
	}

	p1 := clampUnit(rate1)
	p2 := clampUnit(rate2)
	n1 := float64(flights1)
	n2 := float64(flights2)
	pooled := (p1*n1 + p2*n2) / (n1 + n2)
	variance := pooled * (1 - pooled) * (1/n1 + 1/n2)

	if variance <= 0 {
		return false
	}

	z := (p1 - p2) / math.Sqrt(variance)
	if z < 0 {
		z = -z
	}

	return z > proportionZCritical
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}

	if v > 1 {
		return 1
	}

	return v
}

// MinConfidence returns the weakest value.
// Unknown is weaker than low, low is weaker than medium, and medium is weaker than high.
// An empty list is unknown. Any other word counts as unknown.
func MinConfidence(values ...string) string {
	if len(values) == 0 {
		return ConfidenceUnknown
	}

	weakest := values[0]
	for _, value := range values[1:] {
		if confidenceRank(value) < confidenceRank(weakest) {
			weakest = value
		}
	}

	return weakest
}

func confidenceRank(confidence string) int {
	switch confidence {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}

// CountConfidence maps a sample onto high, medium, low, or unknown.
// A non-positive count is unknown. Counts below lowBelow are low, and counts below highAt are medium.
// lowBelow <= 0 skips the low band. highAt <= lowBelow skips the medium band.
func CountConfidence(count, lowBelow, highAt int) string {
	if count <= 0 {
		return ConfidenceUnknown
	}

	if lowBelow > 0 && count < lowBelow {
		return ConfidenceLow
	}

	if highAt > lowBelow && count < highAt {
		return ConfidenceMedium
	}

	return ConfidenceHigh
}

// ApplyOutlookSample sets sample_reason and confidence from the slot size.
// sufficient covers medium and high, and connection thresholds stay.
// insufficient_sample is low confidence: estimates stay and connection thresholds are cleared.
// empty_sample clears estimates so a zero is not a forecast.
func (o *RouteOutlook) ApplyOutlookSample() {
	if o == nil {
		return
	}

	o.Analysis.Confidence = CountConfidence(o.Analysis.SampleSize, OutlookLowSampleBelow, OutlookHighSampleAt)

	switch o.Analysis.Confidence {
	case ConfidenceHigh, ConfidenceMedium:
		o.Analysis.SampleReason = SampleReasonSufficient
	case ConfidenceLow:
		o.Analysis.SampleReason = SampleReasonInsufficientSample
		o.Connection = ConnectionGuidance{}
	default:
		if o.Analysis.SampleSize < 0 {
			o.Analysis.SampleSize = 0
		}

		o.Analysis.SampleReason = SampleReasonEmptySample
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
	o.MedianDepartureDelayMinutes = nil
	o.LikelyDepartureDelayWhenDelayed = nil
	o.MedianDepartureDelayWhenDelayed = nil
	o.Connection = ConnectionGuidance{}
}

// DecisionConfidence is the confidence value an itinerary copies from this outlook.
// A recognized label is kept. Anything else is derived from the sample fields.
func (o *RouteOutlook) DecisionConfidence() string {
	if o == nil {
		return ConfidenceUnknown
	}

	switch o.Analysis.Confidence {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow, ConfidenceUnknown:
		return o.Analysis.Confidence
	default:
		if o.Analysis.SampleSize <= 0 {
			return ConfidenceUnknown
		}

		if o.Analysis.SampleReason == SampleReasonInsufficientSample {
			return ConfidenceLow
		}

		return CountConfidence(o.Analysis.SampleSize, OutlookLowSampleBelow, OutlookHighSampleAt)
	}
}

// BlockingSampleReason is empty_sample or insufficient_sample when that sample
// withholds connection thresholds. A sufficient sample returns an empty string.
// An explicit sample_reason wins. Otherwise a low-confidence non-empty sample
// is insufficient_sample, and any other empty sample is empty_sample.
// Nil returns an empty string. A missing inbound outlook is not a sample reason.
func (o *RouteOutlook) BlockingSampleReason() string {
	if o == nil {
		return ""
	}

	switch reason := o.resolvedSampleReason(); reason {
	case SampleReasonEmptySample, SampleReasonInsufficientSample:
		return reason
	default:
		return ""
	}
}

// resolvedSampleReason is the outlook sample_reason, or the same words derived
// from the sample size and confidence when the field was not set.
func (o *RouteOutlook) resolvedSampleReason() string {
	switch o.Analysis.SampleReason {
	case SampleReasonSufficient, SampleReasonInsufficientSample, SampleReasonEmptySample:
		return o.Analysis.SampleReason
	}

	if o.Analysis.SampleSize > 0 && o.Analysis.Confidence == ConfidenceLow {
		return SampleReasonInsufficientSample
	}

	if o.Analysis.SampleSize <= 0 && o.Analysis.Confidence != ConfidenceHigh {
		return SampleReasonEmptySample
	}

	return SampleReasonSufficient
}

// SeasonalConfidence is high when flights meet minSample, low when some flights fall short, and unknown at zero.
// It has no medium band. Month confidence uses CountConfidence with the history bands.
func SeasonalConfidence(flights, minSample int) string {
	return CountConfidence(flights, minSample, minSample)
}

// SeasonalReliability classifies a rounded on-time rate. Medium and high confidence both count.
func SeasonalReliability(onTimeRate float64, confidence string) string {
	if confidence != ConfidenceHigh && confidence != ConfidenceMedium {
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
