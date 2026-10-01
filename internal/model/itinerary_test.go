package model

import (
	"strings"
	"testing"
)

const (
	sampleOrigin = "BOS"
	sampleDest   = "DEN"
	sampleNext   = "SFO"
	sampleDate   = "2026-10-06"
	sampleArr    = "0905"
)

func TestItineraryOutlookRequestValidate(t *testing.T) {
	window := 45
	ok := ItineraryOutlookRequest{Legs: []ItineraryLeg{
		{Origin: "bos", Dest: " den ", Carrier: "ua", Date: sampleDate, DepTime: "700", ArrTime: sampleArr},
		{Origin: sampleDest, Dest: sampleNext, Carrier: "UA", Date: sampleDate, DepTime: "1100", ArrTime: "1330", DepTimeWindowMinutes: &window},
	}}

	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if ok.Legs[0].Origin != sampleOrigin || ok.Legs[0].Dest != sampleDest || ok.Legs[0].Carrier != "UA" {
		t.Fatalf("normalized = %+v", ok.Legs[0])
	}

	tests := []struct {
		name    string
		req     ItineraryOutlookRequest
		wantErr string
	}{
		{
			name:    "one leg",
			req:     ItineraryOutlookRequest{Legs: []ItineraryLeg{ok.Legs[0]}},
			wantErr: "legs must contain at least 2 items",
		},
		{
			name: "five legs",
			req: ItineraryOutlookRequest{Legs: []ItineraryLeg{
				ok.Legs[0], ok.Legs[1], ok.Legs[0], ok.Legs[1], ok.Legs[0],
			}},
			wantErr: "legs must contain at most 4 items",
		},
		{
			name: "bad time",
			req: ItineraryOutlookRequest{Legs: []ItineraryLeg{
				{Origin: sampleOrigin, Dest: sampleDest, Carrier: "UA", Date: sampleDate, DepTime: "2500", ArrTime: sampleArr},
				ok.Legs[1],
			}},
			wantErr: "dep_time must be a valid local time (hhmm)",
		},
		{
			name: "bad date",
			req: ItineraryOutlookRequest{Legs: []ItineraryLeg{
				{Origin: sampleOrigin, Dest: sampleDest, Carrier: "UA", Date: "2026-13-01", DepTime: "0700", ArrTime: sampleArr},
				ok.Legs[1],
			}},
			wantErr: "date must be a valid date (YYYY-MM-DD)",
		},
		{
			name: "window too wide",
			req: ItineraryOutlookRequest{Legs: []ItineraryLeg{
				ok.Legs[0],
				{Origin: sampleDest, Dest: sampleNext, Carrier: "UA", Date: sampleDate, DepTime: "1100", ArrTime: "1330", DepTimeWindowMinutes: intPtr(121)},
			}},
			wantErr: "dep_time_window_minutes must be <= 120",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err == nil {
				t.Fatal("Validate() expected error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
}
