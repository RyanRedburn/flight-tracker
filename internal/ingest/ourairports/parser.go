package ourairports

import (
	"io"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/csvparse"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func ParseCSV(r io.Reader, dataset store.OurAirportsDataset) (columns []string, rows [][]string, err error) {
	cols, err := Columns(dataset)
	if err != nil {
		return nil, nil, err
	}

	return csvparse.Parse(r, cols, identityHeader)
}

func identityHeader(raw string) string {
	return strings.TrimSpace(raw)
}
