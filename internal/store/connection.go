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
// A missing bucket is still published, with floor_only true. A stored minute, including zero, is floor_only false.
// loose_minutes is that recommendation plus 30. A nil p90 or an insufficient sample returns nulls,
// which is not floor-only: those buckets were not estimated.
func ConnectionRecommendation(insufficient bool, arrivalDelayP90 *float64, mct *AirportConnectionMCT) model.ConnectionGuidance {
	if insufficient || arrivalDelayP90 == nil {
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
	floor, floorOnly := connectionFloor(minutes, fallback)
	out := model.NewConnectionMinutes(delay + floor)
	out.FloorOnly = floorOnly

	return out
}

func connectionFloor(minutes *int, fallback int) (int, bool) {
	if minutes == nil {
		return fallback, true
	}

	return *minutes, false
}
