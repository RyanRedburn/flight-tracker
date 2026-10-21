package mcp

import (
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/jsonschema-go/jsonschema"
)

const (
	patternIATACode    = `^[A-Za-z]{3}$`
	patternCarrierCode = `^[A-Za-z0-9]{2}$`
	patternISODate     = `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`

	isoDateLen = 10

	msgDatePair = "Pass both start_date and end_date, or neither."
	msgRoute    = "Origin and destination are required."
)

func limitExact(prop *jsonschema.Schema, length int, pattern string) {
	prop.MinLength = jsonschema.Ptr(length)
	prop.MaxLength = jsonschema.Ptr(length)
	prop.Pattern = pattern
}

func limitDate(prop *jsonschema.Schema) {
	prop.MinLength = jsonschema.Ptr(isoDateLen)
	prop.MaxLength = jsonschema.Ptr(isoDateLen)
	prop.Pattern = patternISODate
}

func requireRoute(origin, dest string) (string, string, *Failure) {
	origin = normalizeIATACode(origin)
	dest = normalizeIATACode(dest)

	if origin == "" || dest == "" {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: msgRoute,
		}
	}

	if len(origin) != 3 {
		return "", "", invalidIATA("origin")
	}

	if len(dest) != 3 {
		return "", "", invalidIATA("dest")
	}

	return origin, dest, nil
}

func optionalCarrier(code string) (string, *Failure) {
	code = normalizeCarrier(code)
	if code == "" {
		return "", nil
	}

	if len(code) != 2 {
		return "", &Failure{Code: CodeInvalidInput, Message: msgCarrierCode}
	}

	return code, nil
}

func optionalStatsDates(start, end string) (string, string, *Failure) {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)

	if start == "" && end == "" {
		return "", "", nil
	}

	if start == "" || end == "" {
		return "", "", &Failure{Code: CodeInvalidInput, Message: msgDatePair}
	}

	if err := (store.RouteStatsFilter{StartDate: start, EndDate: end}).Validate(); err != nil {
		return "", "", invalidInput(err)
	}

	return start, end, nil
}
