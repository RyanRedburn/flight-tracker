package iem

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	testdataRowCount      = 10
	testStationORD        = "ORD"
	testStationJFK        = "JFK"
	testStationATL        = "ATL"
	testStationDEN        = "DEN"
	testStationSFO        = "SFO"
	testStationXYZ        = "XYZ"
	testNetworkILASOS     = "IL_ASOS"
	testNetworkNYASOS     = "NY_ASOS"
	testValidTimestamp    = "2024-01-01T00:51:00Z"
	testGeoJSONProperties = "properties"
	testGeoJSONSID        = "sid"
)

func fixtureCSVPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "testdata", "asos_2024_01.csv")

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

	return func(context.Context, int, int, []string) (string, func(), error) {
		return path, func() {}, nil
	}
}
