package model

const (
	AirportMatchExactIATA = "exact_iata"
	AirportMatchCity      = "city"
	AirportMatchName      = "name"
	AirportMatchCloseCode = "close_code"
)

// Airport is one row from the airports reference table.
type Airport struct {
	IATACode     string `json:"iata_code"`
	Name         string `json:"name"`
	Municipality string `json:"municipality,omitempty"`
	ISOCountry   string `json:"iso_country,omitempty"`
	ISORegion    string `json:"iso_region,omitempty"`
	Type         string `json:"type,omitempty"`
	Match        string `json:"match,omitempty" enums:"exact_iata,city,name,close_code"`
}
