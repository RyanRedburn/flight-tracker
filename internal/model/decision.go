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

	// ComparisonRanked means the item with the highest 95% Wilson lower bound differs at 95% from at least one other item.
	// ComparisonTooCloseToCall means that item is inside the margin of error of every other item.
	// ComparisonSingle means one item, so there is no best or worst list.
	ComparisonRanked         = "ranked"
	ComparisonTooCloseToCall = "too_close_to_call"
	ComparisonSingle         = "single"

	// proportionZCritical is the two-sided 95% critical value for a pooled two-proportion z-test.
	proportionZCritical = 1.96

	// fisherAlpha is the two-sided 0.05 threshold for the Fisher exact test.
	fisherAlpha = 0.05

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
	// empty_sample is the inbound outlook sample_reason when that slot has no flights.
	// A low sample is still classified. missing_outlook is a null inbound outlook
	// (route outlook 404 / leg error), which is not empty_sample.
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

	// CarrierLowSampleBelow and CarrierHighSampleAt are the carrier bands.
	// Below 100 is low, 100–299 is medium, and 300 or more is high.
	CarrierLowSampleBelow = 100
	CarrierHighSampleAt   = 300

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

// ProportionsDiffer reports whether two on-time rates differ at 95%.
// Counts come from the rates and flight totals. A non-positive sample does not differ.
// A two-sided Fisher exact test is used when any expected cell is below 5.
// Otherwise the comparison is a pooled two-proportion z-test.
func ProportionsDiffer(rate1 float64, flights1 int, rate2 float64, flights2 int) bool {
	if flights1 <= 0 || flights2 <= 0 {
		return false
	}

	p1 := clampUnit(rate1)
	p2 := clampUnit(rate2)
	on1 := countFromRate(p1, flights1)
	on2 := countFromRate(p2, flights2)

	if expectedCellBelowFive(on1, flights1-on1, on2, flights2-on2) {
		return fisherTwoSided(on1, flights1-on1, on2, flights2-on2) < fisherAlpha
	}

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

func countFromRate(rate float64, flights int) int {
	count := int(math.Round(rate * float64(flights)))
	if count < 0 {
		return 0
	}

	if count > flights {
		return flights
	}

	return count
}

func expectedCellBelowFive(on1, off1, on2, off2 int) bool {
	n1 := float64(on1 + off1)
	n2 := float64(on2 + off2)
	on := float64(on1 + on2)
	off := float64(off1 + off2)
	total := n1 + n2

	for _, expected := range []float64{n1 * on / total, n1 * off / total, n2 * on / total, n2 * off / total} {
		if expected < 5 {
			return true
		}
	}

	return false
}

// fisherTwoSided is the two-sided Fisher exact p-value for a 2x2 table.
// The table is on-time and not on-time for each sample. Probabilities at or
// below the observed table are summed.
func fisherTwoSided(on1, off1, on2, off2 int) float64 {
	n1 := on1 + off1
	on := on1 + on2
	total := n1 + on2 + off2
	lo := max(0, on-(total-n1))
	hi := min(on, n1)
	observed := logHypergeometric(on1, n1, on, total)
	sum := 0.0

	for count := lo; count <= hi; count++ {
		lp := logHypergeometric(count, n1, on, total)
		if lp <= observed+1e-10 {
			sum += math.Exp(lp)
		}
	}

	if sum > 1 {
		return 1
	}

	return sum
}

func logHypergeometric(count, draw, successes, total int) float64 {
	return logChoose(draw, count) + logChoose(total-draw, successes-count) - logChoose(total, successes)
}

func logChoose(n, k int) float64 {
	if k < 0 || k > n {
		return math.Inf(-1)
	}

	if k == 0 || k == n {
		return 0
	}

	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))

	return a - b - c
}

// ComparisonRate prefers an on-time count over a rounded rate.
func ComparisonRate(rate float64, onTimeCount *int, flights int) float64 {
	if onTimeCount != nil && flights > 0 {
		return float64(*onTimeCount) / float64(flights)
	}

	return rate
}

// WilsonBounds is the 95% Wilson score interval for an on-time proportion.
// The lower bound orders a leader. The upper bound orders a worst anchor.
// Ties still use ProportionsDiffer. A non-positive sample is 0, 0.
func WilsonBounds(rate float64, onTimeCount *int, flights int) (lower, upper float64) {
	if flights <= 0 {
		return 0, 0
	}

	p := clampUnit(ComparisonRate(rate, onTimeCount, flights))
	n := float64(flights)
	z2 := proportionZCritical * proportionZCritical
	denom := 1 + z2/n
	center := (p + z2/(2*n)) / denom
	margin := proportionZCritical * math.Sqrt(p*(1-p)/n+z2/(4*n*n)) / denom

	return clampUnit(center - margin), clampUnit(center + margin)
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
// sufficient covers medium and high. insufficient_sample is low confidence.
// Estimates and connection thresholds stay for a low sample.
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
	o.OnTimeCount = nil
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

// BlockingSampleReason is empty_sample when the slot has no flights.
// A low sample does not block connection guidance. Nil returns an empty string.
// A missing inbound outlook is not a sample reason.
func (o *RouteOutlook) BlockingSampleReason() string {
	if o == nil {
		return ""
	}

	if o.resolvedSampleReason() == SampleReasonEmptySample {
		return SampleReasonEmptySample
	}

	return ""
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

	if o.Analysis.SampleSize <= 0 && o.Analysis.Confidence != ConfidenceHigh && o.Analysis.Confidence != ConfidenceMedium {
		return SampleReasonEmptySample
	}

	return SampleReasonSufficient
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
// Confidence uses the outlook bands. The reliability word stays unknown at low confidence.
func SeasonalDecision(flights int, onTimeRate float64) (confidence, reliability string) {
	confidence = CountConfidence(flights, OutlookLowSampleBelow, OutlookHighSampleAt)
	reliability = SeasonalReliability(onTimeRate, confidence)

	return confidence, reliability
}
