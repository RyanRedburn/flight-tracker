package handlers

import (
	"encoding/json"
	"errors"
	"io"
)

type jsonBodyOptions struct {
	disallowUnknown bool
	rejectTrailing  bool
}

func decodeJSONBody(body io.Reader, dest any, opts jsonBodyOptions) error {
	decoder := json.NewDecoder(body)
	if opts.disallowUnknown {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(dest); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}

	if !opts.rejectTrailing {
		return nil
	}

	if err := decoder.Decode(&struct{}{}); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}

	return errors.New("unexpected trailing json")
}
