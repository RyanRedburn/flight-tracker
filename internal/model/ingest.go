package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

const MinBTSIngestYear = 2018

var ingestValidate = validator.New()

func init() {
	ingestValidate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name, _, _ := strings.Cut(fld.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			return name
		}

		return strings.ToLower(fld.Name)
	})
	ingestValidate.RegisterStructValidation(validateIngestRequest, IngestRequest{})
}

type ForceIngestRequest struct {
	Force bool `json:"force"`
}

type IngestRequest struct {
	StartYear  int  `json:"start_year" validate:"required,gte=2018"`
	StartMonth int  `json:"start_month" validate:"required,gte=1,lte=12"`
	EndYear    *int `json:"end_year,omitempty" validate:"omitempty,gte=2018"`
	EndMonth   *int `json:"end_month,omitempty" validate:"omitempty,gte=1,lte=12"`
	Force      bool `json:"force"`
}

func (r IngestRequest) Validate() error {
	if err := ingestValidate.Struct(r); err != nil {
		return formatIngestValidationError(err)
	}

	return nil
}

func validateIngestRequest(sl validator.StructLevel) {
	req := sl.Current().Interface().(IngestRequest)

	if (req.EndYear == nil) != (req.EndMonth == nil) {
		sl.ReportError(req.EndYear, "EndYear", "end_year", "end_pair", "")
	}
}

func formatIngestValidationError(err error) error {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	messages := make([]string, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		messages = append(messages, formatIngestFieldError(fieldErr))
	}

	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func formatIngestFieldError(fieldErr validator.FieldError) string {
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
	case "end_pair":
		return "end_year and end_month must both be set or both omitted"
	default:
		return field + " is invalid"
	}
}
