package store

import (
	"errors"
	"fmt"
)

type OurAirportsDataset string

const (
	OurAirportsCountries OurAirportsDataset = "countries"
	OurAirportsRegions   OurAirportsDataset = "regions"
	OurAirportsAirports  OurAirportsDataset = "airports"
)

var ErrInvalidOurAirportsDataset = errors.New("invalid ourairports dataset")

func (d OurAirportsDataset) Table() (string, error) {
	switch d {
	case OurAirportsCountries:
		return "countries", nil
	case OurAirportsRegions:
		return "regions", nil
	case OurAirportsAirports:
		return "airports", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidOurAirportsDataset, d)
	}
}
