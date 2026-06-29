package source

import (
	"context"
	"encoding/json"
)

type FetchRequest struct {
	Params json.RawMessage `json:"params,omitempty"`
}

type Provider interface {
	Fetch(ctx context.Context, req FetchRequest) (json.RawMessage, error)
}
