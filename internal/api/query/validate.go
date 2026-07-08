package query

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func init() {
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		for _, tag := range []string{"query", "json"} {
			name, _, _ := strings.Cut(fld.Tag.Get(tag), ",")
			if name != "" && name != "-" {
				return name
			}
		}

		return strings.ToLower(fld.Name)
	})

	validate.RegisterAlias("list_limit", "gte=1,lte="+strconv.Itoa(MaxListLimit))
}

func Validate(s any) error {
	if err := validate.Struct(s); err != nil {
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
		messages = append(messages, formatValidationFieldError(fieldErr))
	}

	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func formatValidationFieldError(fieldErr validator.FieldError) string {
	field := fieldErr.Field()

	switch fieldErr.Tag() {
	case "len":
		return field + " must be exactly " + fieldErr.Param() + " characters"
	case "datetime":
		return field + " must be a valid date (YYYY-MM-DD)"
	case "gte":
		return field + " must be >= " + fieldErr.Param()
	case "lte":
		return field + " must be <= " + fieldErr.Param()
	case "required":
		return field + " is required"
	case "required_with":
		return field + " is required when " + toQueryLabel(fieldErr.Param()) + " is set"
	case "gtefield":
		return field + " must be on or after " + toQueryLabel(fieldErr.Param())
	case "list_limit":
		return fmt.Sprintf("%s must be between 1 and %d", field, MaxListLimit)
	case "date_span":
		return "date range must be at most " + fieldErr.Param() + " days"
	case "hhmm":
		return field + " must be a valid local time (hhmm)"
	default:
		return field + " is invalid"
	}
}

func toQueryLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "_") || strings.ToLower(name) == name {
		return strings.ToLower(name)
	}

	var b strings.Builder

	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}

		b.WriteRune(r)
	}

	return strings.ToLower(b.String())
}
