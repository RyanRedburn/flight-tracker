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
// A nil p90 or an insufficient sample returns nulls.
func ConnectionRecommendation(insufficient bool, arrivalDelayP90 *float64, mct *AirportConnectionMCT) model.RecommendedConnectionMinutes {
	if insufficient || arrivalDelayP90 == nil {
		return model.RecommendedConnectionMinutes{}
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

	dd := delay + connectionFloor(domesticToDomestic, connectionFloorDomesticToDomestic)
	di := delay + connectionFloor(domesticToInternational, connectionFloorDomesticToInternational)
	id := delay + connectionFloor(internationalToDomestic, connectionFloorInternationalToDomestic)
	ii := delay + connectionFloor(internationalToInternational, connectionFloorInternationalToInternational)

	return model.RecommendedConnectionMinutes{
		DomesticToDomestic:           &dd,
		DomesticToInternational:      &di,
		InternationalToDomestic:      &id,
		InternationalToInternational: &ii,
	}
}

func connectionFloor(minutes *int, fallback int) int {
	if minutes == nil {
		return fallback
	}

	return *minutes
}
