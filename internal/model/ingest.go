package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

const MinFlightPerformanceIngestYear = 2018

// MinWeatherIngestYear matches BTS flight-performance coverage.
const MinWeatherIngestYear = 2018

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
	ingestValidate.RegisterStructValidation(validateWeatherIngestRequest, WeatherIngestRequest{})
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

// WeatherIngestRequest queues ASOS/METAR observation imports.
// Omit stations to auto-resolve from distinct BTS origin/dest ∩ US IEM ASOS metadata.
type WeatherIngestRequest struct {
	StartYear  int      `json:"start_year" validate:"required,gte=2018"`
	StartMonth int      `json:"start_month" validate:"required,gte=1,lte=12"`
	EndYear    *int     `json:"end_year,omitempty" validate:"omitempty,gte=2018"`
	EndMonth   *int     `json:"end_month,omitempty" validate:"omitempty,gte=1,lte=12"`
	Stations   []string `json:"stations,omitempty" validate:"omitempty,dive,required,min=3,max=4"`
	Force      bool     `json:"force"`
}

func (r IngestRequest) Validate() error {
	if err := ingestValidate.Struct(r); err != nil {
		return formatIngestValidationError(err)
	}

	return nil
}

func (r *WeatherIngestRequest) Validate() error {
	normalizeWeatherStations(r)

	if err := ingestValidate.Struct(r); err != nil {
		return formatIngestValidationError(err)
	}

	return nil
}

func normalizeWeatherStations(r *WeatherIngestRequest) {
	if r == nil {
		return
	}

	if len(r.Stations) == 0 {
		r.Stations = nil
		return
	}

	out := make([]string, 0, len(r.Stations))
	seen := make(map[string]struct{}, len(r.Stations))

	for _, station := range r.Stations {
		station = strings.ToUpper(strings.TrimSpace(station))
		if station == "" {
			continue
		}

		if _, ok := seen[station]; ok {
			continue
		}

		seen[station] = struct{}{}
		out = append(out, station)
	}

	if len(out) == 0 {
		r.Stations = nil
		return
	}

	r.Stations = out
}

func validateIngestRequest(sl validator.StructLevel) {
	req := sl.Current().Interface().(IngestRequest)

	if (req.EndYear == nil) != (req.EndMonth == nil) {
		sl.ReportError(req.EndYear, "EndYear", "end_year", "end_pair", "")
	}
}

func validateWeatherIngestRequest(sl validator.StructLevel) {
	req := sl.Current().Interface().(WeatherIngestRequest)

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
	case "min":
		if fieldErr.Type().Kind() == reflect.Slice || fieldErr.Type().Kind() == reflect.Array {
			return field + " must contain at least " + fieldErr.Param() + " items"
		}

		return field + " must be at least " + fieldErr.Param() + " characters"
	case "max":
		if fieldErr.Type().Kind() == reflect.Slice || fieldErr.Type().Kind() == reflect.Array {
			return field + " must contain at most " + fieldErr.Param() + " items"
		}

		return field + " must be at most " + fieldErr.Param() + " characters"
	case "required":
		return field + " is required"
	case "end_pair":
		return errEndYearMonthPair
	default:
		return field + " is invalid"
	}
}

const errEndYearMonthPair = "end_year and end_month must both be set or both omitted"
