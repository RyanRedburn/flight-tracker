package operator

import (
	"context"
	"encoding/json"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

type JobHandler interface {
	Type() string
	Process(ctx context.Context, job *model.Job) (json.RawMessage, error)
}
