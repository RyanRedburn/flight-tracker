package bts

import (
	"errors"
	"path/filepath"
	"runtime"
)

// TestdataRowCount is the number of data rows in the checked-in BTS sample CSV.
const TestdataRowCount = 20

// TestdataCSV returns the absolute path to the checked-in BTS sample CSV used in tests.
func TestdataCSV() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "testdata", "on_time_2026_04.csv")

	return filepath.Abs(path)
}
