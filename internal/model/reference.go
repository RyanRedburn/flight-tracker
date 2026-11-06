package model

import (
	"errors"
	"fmt"
)

type ReferenceDataset string

const (
	ReferenceCountries  ReferenceDataset = "countries"
	ReferenceRegions    ReferenceDataset = "regions"
	ReferenceAirports   ReferenceDataset = "airports"
	ReferenceAirportMCT ReferenceDataset = "airport_mct"
)

var ErrInvalidReferenceDataset = errors.New("invalid reference dataset")

func (d ReferenceDataset) JobType() (JobType, error) {
	switch d {
	case ReferenceCountries:
		return JobTypeImportCountries, nil
	case ReferenceRegions:
		return JobTypeImportRegions, nil
	case ReferenceAirports:
		return JobTypeImportAirports, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidReferenceDataset, d)
	}
}
