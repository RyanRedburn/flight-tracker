package query

import (
	"strings"
	"testing"
)

func TestValidateMessages(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name: "required uses query tag",
			input: struct {
				Origin string `query:"origin" validate:"required"`
			}{},
			wantErr: "origin is required",
		},
		{
			name: "len",
			input: struct {
				Origin string `query:"origin" validate:"len=3"`
			}{Origin: "AB"},
			wantErr: "origin must be exactly 3 characters",
		},
		{
			name: "required_with",
			input: struct {
				Carrier      string `query:"carrier" validate:"required_with=FlightNumber,omitempty,len=2"`
				FlightNumber string `query:"flight_number"`
			}{FlightNumber: "100"},
			wantErr: "carrier is required when flight_number is set",
		},
		{
			name: "list_limit",
			input: struct {
				Limit int `query:"limit" validate:"list_limit"`
			}{Limit: 501},
			wantErr: "limit must be between 1 and 500",
		},
		{
			name: "hhmm",
			input: struct {
				DepTime string `query:"dep_time" validate:"hhmm"`
			}{DepTime: "2500"},
			wantErr: "dep_time must be a valid local time (hhmm)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if err == nil {
				t.Fatal("Validate() expected error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
