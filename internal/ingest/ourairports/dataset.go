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

func Columns(dataset store.OurAirportsDataset) ([]string, error) {
	switch dataset {
	case store.OurAirportsCountries:
		return append([]string(nil), countryColumns...), nil
	case store.OurAirportsRegions:
		return append([]string(nil), regionColumns...), nil
	case store.OurAirportsAirports:
		return append([]string(nil), airportColumns...), nil
	default:
		return nil, fmt.Errorf("%w: %q", store.ErrInvalidOurAirportsDataset, dataset)
	}
}

func JobType(dataset store.OurAirportsDataset) (string, error) {
	switch dataset {
	case store.OurAirportsCountries:
		return model.JobTypeImportOurAirportsCountries, nil
	case store.OurAirportsRegions:
		return model.JobTypeImportOurAirportsRegions, nil
	case store.OurAirportsAirports:
		return model.JobTypeImportOurAirportsAirports, nil
	default:
		return "", fmt.Errorf("%w: %q", store.ErrInvalidOurAirportsDataset, dataset)
	}
}

func CSVFilename(dataset store.OurAirportsDataset) (string, error) {
	table, err := dataset.Table()
	if err != nil {
		return "", err
	}

	return table + ".csv", nil
}
