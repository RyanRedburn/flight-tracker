package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var onTimeFlightFilterValidator = validator.New()

func (f OnTimeFlightFilter) Validate() error {
	if err := onTimeFlightFilterValidator.Struct(f); err != nil {
		return formatValidationError(err)
	}

	return nil
}

func formatValidationError(err error) error {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	messages := make([]string, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		messages = append(messages, formatFieldError(fieldErr))
	}

	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func formatFieldError(fieldErr validator.FieldError) string {
	field := strings.ToLower(fieldErr.Field())

	switch fieldErr.Tag() {
	case "len":
		return fmt.Sprintf("%s must be exactly 3 characters", field)
	case "datetime":
		return fmt.Sprintf("%s must be a valid date (YYYY-MM-DD)", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
