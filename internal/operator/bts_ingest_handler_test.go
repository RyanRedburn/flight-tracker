package operator

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func newTestBTSIngestHandler(t *testing.T, store *mem.Store) *BTSIngestHandler {
	t.Helper()

	path, err := bts.TestdataCSV()
	if err != nil {
		t.Fatalf("TestdataCSV() error = %v", err)
	}

	opener := func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	}

	svc := bts.NewService(store, nil).WithCSVOpener(opener)

	return NewBTSIngestHandler(store, svc)
}
