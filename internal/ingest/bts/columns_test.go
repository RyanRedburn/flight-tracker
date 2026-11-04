package bts

import "testing"

func TestCSVHeaderToColumnKnownFields(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{header: "Year", want: colYear},
		{header: "FlightDate", want: "flight_date"},
		{header: "DayofMonth", want: colDayOfMonth},
		{header: "Operating_Airline ", want: "operating_airline"},
		{header: "OriginAirportID", want: "origin_airport_id"},
		{header: "CRSDepTime", want: "crs_dep_time"},
		{header: "Duplicate", want: "duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := csvHeaderToColumn(tt.header); got != tt.want {
				t.Errorf("csvHeaderToColumn(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
