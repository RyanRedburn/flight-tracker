package store

import (
	"math"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	connectionFloorDomesticToDomestic           = 45
	connectionFloorDomesticToInternational      = 60
	connectionFloorInternationalToDomestic      = 120
	connectionFloorInternationalToInternational = 120
)

// AirportConnectionMCT is the destination airport's stored MCT minutes.
// A nil field means that bucket was not present and the static floor applies.
type AirportConnectionMCT struct {
	DomesticToDomestic           *int
	DomesticToInternational      *int
	InternationalToDomestic      *int
	InternationalToInternational *int
}

// ConnectionRecommendation is max(0, round(arrivalDelayP90)) + floor per connection type.
// floor is the airport MCT minute when that bucket is set, otherwise 45, 60, 120, or 120.
// mct_source is airport for a stored minute, including zero, and default when the bucket is missing.
// loose_minutes is that recommendation plus 30. A nil p90 returns nulls, which were not estimated.
func ConnectionRecommendation(arrivalDelayP90 *float64, mct *AirportConnectionMCT) model.ConnectionGuidance {
	if arrivalDelayP90 == nil {
		return model.ConnectionGuidance{}
	}

	delay := max(0, int(math.Round(*arrivalDelayP90)))

	var (
		domesticToDomestic           *int
		domesticToInternational      *int
		internationalToDomestic      *int
		internationalToInternational *int
	)

	if mct != nil {
		domesticToDomestic = mct.DomesticToDomestic
		domesticToInternational = mct.DomesticToInternational
		internationalToDomestic = mct.InternationalToDomestic
		internationalToInternational = mct.InternationalToInternational
	}

	return model.ConnectionGuidance{
		DomesticToDomestic:           connectionBucket(delay, domesticToDomestic, connectionFloorDomesticToDomestic),
		DomesticToInternational:      connectionBucket(delay, domesticToInternational, connectionFloorDomesticToInternational),
		InternationalToDomestic:      connectionBucket(delay, internationalToDomestic, connectionFloorInternationalToDomestic),
		InternationalToInternational: connectionBucket(delay, internationalToInternational, connectionFloorInternationalToInternational),
	}
}

func connectionBucket(delay int, minutes *int, fallback int) *model.ConnectionMinutes {
	floor, source := connectionFloor(minutes, fallback)
	out := model.NewConnectionMinutes(delay + floor)
	out.MCTSource = source

	return out
}

func connectionFloor(minutes *int, fallback int) (int, string) {
	if minutes == nil {
		return fallback, model.MCTSourceDefault
	}

	return *minutes, model.MCTSourceAirport
}
