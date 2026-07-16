package ingest

import (
	"context"
)

// FileOpener resolves a local CSV file path for parsing.
type FileOpener func(ctx context.Context) (path string, cleanup func(), err error)
