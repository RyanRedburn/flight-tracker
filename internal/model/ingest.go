package model

import (
	"github.com/RyanRedburn/flight-tracker/internal/validation"

	"github.com/go-playground/validator/v10"
)

const MinBTSIngestYear = 2018

type IngestRequest struct {
	StartYear  int  `json:"start_year" validate:"required,gte=2018"`
	StartMonth int  `json:"start_month" validate:"required,gte=1,lte=12"`
	EndYear    *int `json:"end_year,omitempty" validate:"omitempty,gte=2018"`
	EndMonth   *int `json:"end_month,omitempty" validate:"omitempty,gte=1,lte=12"`
	Force      bool `json:"force"`
}

func (r IngestRequest) Validate() error {
	return validation.Validate(r)
}

func init() {
	validation.RegisterStructValidation(validateIngestRequest, IngestRequest{})
}

func validateIngestRequest(sl validator.StructLevel) {
	req := sl.Current().Interface().(IngestRequest)

	if (req.EndYear == nil) != (req.EndMonth == nil) {
		sl.ReportError(req.EndYear, "EndYear", "end_year", "end_pair", "")
	}
}
