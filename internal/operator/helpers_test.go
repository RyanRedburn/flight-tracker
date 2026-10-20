package operator

import (
	"context"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	testJobID      = "job-1"
	testStationORD = "ORD"
)

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

func rejectRebuild(t *testing.T) func(context.Context) (*model.Job, error) {
	t.Helper()

	return func(context.Context) (*model.Job, error) {
		t.Fatal("ingest queued a rollup rebuild")

		return nil, nil
	}
}
