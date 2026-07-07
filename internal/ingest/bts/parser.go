package bts

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

func ParseCSV(r io.Reader) (columns []string, rows [][]string, err error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read csv header: %w", err)
	}

	indexByColumn, err := buildColumnIndex(header)
	if err != nil {
		return nil, nil, err
	}

	columns = append([]string(nil), DBColumns...)
	rows = make([][]string, 0, 1024)

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, nil, fmt.Errorf("read csv row: %w", err)
		}

		row := make([]string, len(DBColumns))
		for i, col := range DBColumns {
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

	return columns, rows, nil
}

func buildColumnIndex(header []string) (map[string]int, error) {
	indexByColumn := make(map[string]int, len(header))

	for i, raw := range header {
		col := csvHeaderToColumn(raw)
		if col == "" {
			continue
		}

		if _, exists := indexByColumn[col]; exists {
			return nil, fmt.Errorf("duplicate csv column %q", strings.TrimSpace(raw))
		}

		indexByColumn[col] = i
	}

	for _, required := range DBColumns {
		if _, ok := indexByColumn[required]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingColumns, required)
		}
	}

	return indexByColumn, nil
}
