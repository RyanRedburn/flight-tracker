package operator

import (
	"context"
	"errors"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// enqueueRebuildJob inserts a pending rollup rebuild. ErrActiveIngestConflict
// means one of that type is already pending, which is enough: the queued job
// runs after this ingest. A running rebuild is not a conflict.
func enqueueRebuildJob(ctx context.Context, create func(context.Context) (*model.Job, error)) error {
	if _, err := create(ctx); err != nil {
		if errors.Is(err, store.ErrActiveIngestConflict) {
			return nil
		}

		return err
	}

	return nil
}
