package model

import (
	"strings"
	"testing"
)

func TestIngestRequestValidate(t *testing.T) {
	endYear := 2026
	endMonth := 3

	tests := []struct {
		name    string
		req     IngestRequest
		wantErr string
	}{
		{
			name: "valid single month",
			req: IngestRequest{
				StartYear:  2026,
				StartMonth: 4,
			},
		},
		{
			name: "valid range",
			req: IngestRequest{
				StartYear:  2026,
				StartMonth: 1,
				EndYear:    &endYear,
				EndMonth:   &endMonth,
			},
		},
		{
			name: "invalid month",
			req: IngestRequest{
				StartYear:  2026,
				StartMonth: 13,
			},
			wantErr: "start_month must be <= 12",
		},
		{
			name: "invalid year",
			req: IngestRequest{
				StartYear:  2017,
				StartMonth: 1,
			},
			wantErr: "start_year must be >= 2018",
		},
		{
			name: "incomplete end",
			req: IngestRequest{
				StartYear:  2026,
				StartMonth: 1,
				EndYear:    &endYear,
			},
			wantErr: errEndYearMonthPair,
		},
		{
			name:    "missing start",
			req:     IngestRequest{},
			wantErr: "start_year is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
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

func TestWeatherIngestRequestValidate(t *testing.T) {
	endYear := 2024
	endMonth := 3

	tests := []struct {
		name    string
		req     WeatherIngestRequest
		wantErr string
		want    []string
	}{
		{
			name: "valid single month",
			req: WeatherIngestRequest{
				StartYear:  2024,
				StartMonth: 1,
				Stations:   []string{"ord", "JFK", "ord"},
			},
			want: []string{"ORD", "JFK"},
		},
		{
			name: "valid range",
			req: WeatherIngestRequest{
				StartYear:  2024,
				StartMonth: 1,
				EndYear:    &endYear,
				EndMonth:   &endMonth,
				Stations:   []string{"ATL"},
			},
			want: []string{"ATL"},
		},
		{
			name: "missing stations",
			req: WeatherIngestRequest{
				StartYear:  2024,
				StartMonth: 1,
			},
		},
		{
			name: "station too short",
			req: WeatherIngestRequest{
				StartYear:  2024,
				StartMonth: 1,
				Stations:   []string{"OR"},
			},
			wantErr: "stations[0] must be at least 3 characters",
		},
		{
			name: "incomplete end",
			req: WeatherIngestRequest{
				StartYear:  2024,
				StartMonth: 1,
				EndYear:    &endYear,
				Stations:   []string{"ORD"},
			},
			wantErr: errEndYearMonthPair,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertWeatherIngestValidation(t, &tt.req, tt.wantErr, tt.want)
		})
	}
}

func assertWeatherIngestValidation(t *testing.T, req *WeatherIngestRequest, wantErr string, want []string) {
	t.Helper()

	err := req.Validate()
	if wantErr != "" {
		if err == nil {
			t.Fatal("Validate() expected error")
		}

		if !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), wantErr)
		}

		return
	}

	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(want) == 0 {
		return
	}

	if len(req.Stations) != len(want) {
		t.Fatalf("Stations = %v, want %v", req.Stations, want)
	}

	for i := range want {
		if req.Stations[i] != want[i] {
			t.Fatalf("Stations = %v, want %v", req.Stations, want)
		}
	}
}
