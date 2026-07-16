package csvparse

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrMissingColumns = errors.New("csv missing required columns")
	ErrUnexpectedCSV  = errors.New("unexpected csv row width")
)

// HeaderMapper converts a raw CSV header cell to a canonical column name.
// Return an empty string to skip a header cell.
type HeaderMapper func(raw string) string

func Parse(r io.Reader, columns []string, mapper HeaderMapper) (cols []string, rows [][]string, err error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read csv header: %w", err)
	}

	indexByColumn, err := buildColumnIndex(header, columns, mapper)
	if err != nil {
		return nil, nil, err
	}

	cols = append([]string(nil), columns...)
	rows = make([][]string, 0, 1024)

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, nil, fmt.Errorf("read csv row: %w", err)
		}

		row := make([]string, len(columns))
		for i, col := range columns {
			idx, ok := indexByColumn[col]
			if !ok {
				return nil, nil, fmt.Errorf("%w: %s", ErrMissingColumns, col)
			}

			if idx >= len(record) {
				return nil, nil, fmt.Errorf("%w: column %s", ErrUnexpectedCSV, col)
			}

			row[i] = record[idx]
		}

		rows = append(rows, row)
	}

	return cols, rows, nil
}

func buildColumnIndex(header []string, columns []string, mapper HeaderMapper) (map[string]int, error) {
	indexByColumn := make(map[string]int, len(header))

	for i, raw := range header {
		col := mapper(raw)
		if col == "" {
			continue
		}

		if _, exists := indexByColumn[col]; exists {
			return nil, fmt.Errorf("duplicate csv column %q", strings.TrimSpace(raw))
		}

		indexByColumn[col] = i
	}

	for _, required := range columns {
		if _, ok := indexByColumn[required]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingColumns, required)
		}
	}

	return indexByColumn, nil
}
