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
			wantErr: "end_year and end_month must both be set or both omitted",
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
