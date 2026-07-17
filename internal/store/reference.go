package store

import (
	"errors"
	"fmt"
)

type ReferenceDataset string

const (
	ReferenceCountries ReferenceDataset = "countries"
	ReferenceRegions   ReferenceDataset = "regions"
	ReferenceAirports  ReferenceDataset = "airports"
)

var ErrInvalidReferenceDataset = errors.New("invalid reference dataset")

func (d ReferenceDataset) Table() (string, error) {
	switch d {
	case ReferenceCountries:
		return "countries", nil
	case ReferenceRegions:
		return "regions", nil
	case ReferenceAirports:
		return "airports", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidReferenceDataset, d)
	}
}
