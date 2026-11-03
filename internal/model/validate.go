package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// JoinValidationErrors turns validator errors into one message separated by "; ".
// Other errors are returned as they are.
func JoinValidationErrors(err error, format func(validator.FieldError) string) error {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	messages := make([]string, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		messages = append(messages, format(fieldErr))
	}

	return fmt.Errorf("%s", strings.Join(messages, "; "))
}
