package validation

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
		for _, tag := range []string{"json", "query"} {
			name, _, _ := strings.Cut(fld.Tag.Get(tag), ",")
			if name != "" && name != "-" {
				return name
			}
		}

		return strings.ToLower(fld.Name)
	})

	_ = validate.RegisterValidation("query_int", func(fl validator.FieldLevel) bool {
		raw := fl.Field().String()
		if raw == "" {
			return true
		}

		_, err := strconv.Atoi(raw)

		return err == nil
	})
}

func Validate(s any) error {
	if err := validate.Struct(s); err != nil {
		return formatError(err)
	}

	return nil
}

func RegisterStructValidation(fn validator.StructLevelFunc, types ...any) {
	for _, t := range types {
		validate.RegisterStructValidation(fn, t)
	}
}

func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

func formatError(err error) error {
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
	field := fieldErr.Field()

	switch fieldErr.Tag() {
	case "len":
		return field + " must be exactly 3 characters"
	case "datetime":
		return field + " must be a valid date (YYYY-MM-DD)"
	case "gte":
		return field + " must be >= " + fieldErr.Param()
	case "lte":
		return field + " must be <= " + fieldErr.Param()
	case "required":
		return field + " is required"
	case "query_int":
		return field + " must be a valid integer"
	case "list_limit":
		// Message must stay in sync with query.MaxListLimit.
		return field + " must be between 1 and 500"
	case "end_pair":
		return "end_year and end_month must both be set or both omitted"
	default:
		return field + " is invalid"
	}
}
