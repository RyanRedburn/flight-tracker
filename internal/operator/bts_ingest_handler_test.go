package operator

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

const btsTestdataRowCount = 20

func btsFixtureCSVPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "..", "ingest", "bts", "testdata", "on_time_2026_04.csv")

	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("bts testdata csv missing at %s: %v", path, err)
	}

	return path
}

func newTestBTSIngestHandler(t *testing.T, store *mem.Store) *BTSIngestHandler {
	t.Helper()

	path := btsFixtureCSVPath(t)

	opener := func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	}

	svc := bts.NewService(store, nil).WithCSVOpener(opener)

	return NewBTSIngestHandler(store, svc)
}
