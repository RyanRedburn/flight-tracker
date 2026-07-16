package operator

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestNewWorkerMinimumConcurrency(t *testing.T) {
	st := &storetest.Stub{}
	processor := NewProcessor(st)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	worker := NewWorker(st, processor, 0, time.Second, logger)
	if worker.concurrency != 1 {
		t.Errorf("concurrency = %d, want 1", worker.concurrency)
	}
}
