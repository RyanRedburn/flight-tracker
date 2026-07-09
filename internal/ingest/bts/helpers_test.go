package bts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const testdataRowCount = 20

func fixtureCSVPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "testdata", "on_time_2026_04.csv")

	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("testdata csv missing at %s: %v", path, err)
	}

	return path
}

func minimalCSVOpener(t *testing.T) CSVOpener {
	t.Helper()

	path := fixtureCSVPath(t)

	return func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	}
}
