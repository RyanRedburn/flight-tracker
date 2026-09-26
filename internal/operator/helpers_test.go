package operator

import (
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const testJobID = "job-1"

func mustNewProcessor(t *testing.T, s store.Store, handlers ...JobHandler) *Processor {
	t.Helper()

	processor, err := NewProcessor(s, handlers...)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	return processor
}

var (
	errRebuildFailed = errors.New("rebuild failed")
	errImportFailed  = errors.New("import failed")
)
