package operator

import "errors"

const testJobID = "job-1"

var (
	errRebuildFailed = errors.New("rebuild failed")
	errImportFailed  = errors.New("import failed")
)
