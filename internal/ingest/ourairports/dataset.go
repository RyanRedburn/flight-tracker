package ourairports

import (
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	colID            = "id"
	colCode          = "code"
	colName          = "name"
	colContinent     = "continent"
	colWikipediaLink = "wikipedia_link"
	colKeywords      = "keywords"
	defaultBaseURL   = "https://raw.githubusercontent.com/davidmegginson/ourairports-data/main"
)

var (
	countryColumns = []string{
		colID, colCode, colName, colContinent, colWikipediaLink, colKeywords,
	}
	regionColumns = []string{
		colID, colCode, "local_code", colName, colContinent, "iso_country", colWikipediaLink, colKeywords,
	}
	airportColumns = []string{
		colID, "ident", "type", colName, "latitude_deg", "longitude_deg", "elevation_ft",
		colContinent, "iso_country", "iso_region", "municipality", "scheduled_service",
		"icao_code", "iata_code", "gps_code", "local_code", "home_link", colWikipediaLink, colKeywords,
	}
)

func Columns(dataset model.ReferenceDataset) ([]string, error) {
	switch dataset {
	case model.ReferenceCountries:
		return append([]string(nil), countryColumns...), nil
	case model.ReferenceRegions:
		return append([]string(nil), regionColumns...), nil
	case model.ReferenceAirports:
		return append([]string(nil), airportColumns...), nil
	default:
		return nil, fmt.Errorf("%w: %q", model.ErrInvalidReferenceDataset, dataset)
	}
}

func CSVFilename(dataset model.ReferenceDataset) (string, error) {
	switch dataset {
	case model.ReferenceCountries, model.ReferenceRegions, model.ReferenceAirports, model.ReferenceAirportMCT:
		return string(dataset) + ".csv", nil
	default:
		return "", fmt.Errorf("%w: %q", model.ErrInvalidReferenceDataset, dataset)
	}
}
