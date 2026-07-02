package store

import (
	"testing"
)

func TestOnTimeFlightFilterValidate(t *testing.T) {
	tests := []struct {
		name    string
		filter  OnTimeFlightFilter
		wantErr bool
	}{
		{
			name:   "empty filter",
			filter: OnTimeFlightFilter{},
		},
		{
			name: "valid filter",
			filter: OnTimeFlightFilter{
				FlightDate: "2026-04-24",
				Origin:     "ORD",
				Dest:       "BHM",
			},
		},
		{
			name: "invalid origin length",
			filter: OnTimeFlightFilter{
				Origin: "OR",
			},
			wantErr: true,
		},
		{
			name: "invalid dest length",
			filter: OnTimeFlightFilter{
				Dest: "BHMM",
			},
			wantErr: true,
		},
		{
			name: "invalid flight date",
			filter: OnTimeFlightFilter{
				FlightDate: "2026-13-40",
			},
			wantErr: true,
		},
		{
			name: "invalid flight date format",
			filter: OnTimeFlightFilter{
				FlightDate: "04-24-2026",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() expected error")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
