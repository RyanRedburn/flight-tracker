package bts

import (
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func normalizeFlightNumbers(rows [][]string) error {
	indexes, err := flightNumberColumnIndexes()
	if err != nil {
		return err
	}

	for i, row := range rows {
		for _, idx := range indexes {
			raw := row[idx]
			if strings.TrimSpace(raw) == "" {
				row[idx] = store.MissingFlightNumber
				continue
			}

			number, err := store.NormalizeFlightNumber(raw)
			if err != nil {
				return fmt.Errorf("row %d %s: %w", i+2, DBColumns[idx], err)
			}

			row[idx] = number
		}
	}

	return nil
}

func flightNumberColumnIndexes() ([]int, error) {
	names := []string{
		colFlightNumberMarketing,
		colFlightNumberOperating,
		colFlightNumberCodeshare,
	}

	indexes := make([]int, 0, len(names))

	for _, name := range names {
		idx := -1

		for i, col := range DBColumns {
			if col == name {
				idx = i
				break
			}
		}

		if idx < 0 {
			return nil, fmt.Errorf("flight number column %s missing", name)
		}

		indexes = append(indexes, idx)
	}

	return indexes, nil
}
