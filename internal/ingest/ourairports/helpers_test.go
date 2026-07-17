package ourairports

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const testdataRowCount = 2

func testdataCSVPath(t *testing.T, dataset store.ReferenceDataset) string {
	t.Helper()

	filename, err := CSVFilename(dataset)
	if err != nil {
		t.Fatalf("CSVFilename() error = %v", err)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "testdata", filename)

	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("testdata csv missing at %s: %v", path, err)
	}

	return path
}

func fixtureCSVOpener(t *testing.T) CSVOpener {
	t.Helper()

	return func(_ context.Context, dataset store.ReferenceDataset) (string, func(), error) {
		return testdataCSVPath(t, dataset), func() {}, nil
	}
}
