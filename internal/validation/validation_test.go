package validation

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func init() {
	_ = RegisterValidation("list_limit", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.Int {
			return false
		}

		n := fl.Field().Int()

		return n >= 1 && n <= 500
	})

	RegisterStructValidation(func(sl validator.StructLevel) {
		req := sl.Current().Interface().(endPairRequest)

		if (req.EndYear == nil) != (req.EndMonth == nil) {
			sl.ReportError(req.EndYear, "EndYear", "end_year", "end_pair", "")
		}
	}, endPairRequest{})
}

type endPairRequest struct {
	EndYear  *int `json:"end_year"`
	EndMonth *int `json:"end_month"`
}

func TestValidateSuccess(t *testing.T) {
	type valid struct {
		Name string `json:"name" validate:"required"`
	}

	if err := Validate(valid{Name: "ok"}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateFieldErrors(t *testing.T) {
	endYear := 2026

	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name: "required uses json tag name",
			input: struct {
				StartYear int `json:"start_year" validate:"required"`
			}{},
			wantErr: "start_year is required",
		},
		{
			name: "required uses query tag name",
			input: struct {
				Limit string `query:"limit" validate:"required"`
			}{},
			wantErr: "limit is required",
		},
		{
			name: "required falls back to lowercased field name",
			input: struct {
				StartYear int `validate:"required"`
			}{},
			wantErr: "startyear is required",
		},
		{
			name: "len",
			input: struct {
				Origin string `json:"origin" validate:"len=3"`
			}{Origin: "AB"},
			wantErr: "origin must be exactly 3 characters",
		},
		{
			name: "datetime",
			input: struct {
				FlightDate string `json:"flight_date" validate:"datetime=2006-01-02"`
			}{FlightDate: "not-a-date"},
			wantErr: "flight_date must be a valid date (YYYY-MM-DD)",
		},
		{
			name: "gte",
			input: struct {
				StartYear int `json:"start_year" validate:"gte=2018"`
			}{StartYear: 2017},
			wantErr: "start_year must be >= 2018",
		},
		{
			name: "lte",
			input: struct {
				StartMonth int `json:"start_month" validate:"lte=12"`
			}{StartMonth: 13},
			wantErr: "start_month must be <= 12",
		},
		{
			name: "query_int invalid",
			input: struct {
				Limit string `json:"limit" validate:"query_int"`
			}{Limit: "abc"},
			wantErr: "limit must be a valid integer",
		},
		{
			name: "query_int empty allowed",
			input: struct {
				Limit string `json:"limit" validate:"query_int"`
			}{Limit: ""},
			wantErr: "",
		},
		{
			name: "list_limit",
			input: struct {
				Limit int `json:"limit" validate:"list_limit"`
			}{Limit: 501},
			wantErr: "limit must be between 1 and 500",
		},
		{
			name: "end_pair",
			input: endPairRequest{
				EndYear: &endYear,
			},
			wantErr: "end_year and end_month must both be set or both omitted",
		},
		{
			name: "default tag message",
			input: struct {
				Email string `json:"email" validate:"email"`
			}{Email: "not-an-email"},
			wantErr: "email is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}

				return
			}

			if err == nil {
				t.Fatal("Validate() expected error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRegisterValidation(t *testing.T) {
	err := RegisterValidation("test_tag", func(fl validator.FieldLevel) bool {
		return fl.Field().String() == "ok"
	})
	if err != nil {
		t.Fatalf("RegisterValidation() error = %v", err)
	}

	type tagged struct {
		Value string `json:"value" validate:"test_tag"`
	}

	if err := Validate(tagged{Value: "bad"}); err == nil {
		t.Fatal("Validate() expected error for custom tag")
	}
}
