package ourairports

import (
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	colID            = "id"
	colCode          = "code"
	colName          = "name"
	colContinent     = "continent"
	colWikipediaLink = "wikipedia_link"
	colKeywords      = "keywords"
	jsonKeyDataset   = "dataset"
	jsonKeyRows      = "rows_imported"
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

func Columns(dataset store.ReferenceDataset) ([]string, error) {
	switch dataset {
	case store.ReferenceCountries:
		return append([]string(nil), countryColumns...), nil
	case store.ReferenceRegions:
		return append([]string(nil), regionColumns...), nil
	case store.ReferenceAirports:
		return append([]string(nil), airportColumns...), nil
	default:
		return nil, fmt.Errorf("%w: %q", store.ErrInvalidReferenceDataset, dataset)
	}
}

func JobType(dataset store.ReferenceDataset) (string, error) {
	switch dataset {
	case store.ReferenceCountries:
		return model.JobTypeImportCountries, nil
	case store.ReferenceRegions:
		return model.JobTypeImportRegions, nil
	case store.ReferenceAirports:
		return model.JobTypeImportAirports, nil
	default:
		return "", fmt.Errorf("%w: %q", store.ErrInvalidReferenceDataset, dataset)
	}
}

func CSVFilename(dataset store.ReferenceDataset) (string, error) {
	table, err := dataset.Table()
	if err != nil {
		return "", err
	}

	return table + ".csv", nil
}
