package store

import (
	"errors"
	"testing"
)

func TestNormalizeFlightNumber(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "123", want: "123"},
		{raw: "0123", want: "123"},
		{raw: " 0042 ", want: "42"},
		{raw: "0001", want: "1"},
		{raw: "00001234", want: "1234"},
		{raw: "1", want: "1"},
		{raw: "9999", want: "9999"},
		{raw: "", wantErr: true},
		{raw: "   ", wantErr: true},
		{raw: "0", wantErr: true},
		{raw: "0000", wantErr: true},
		{raw: "12345", wantErr: true},
		{raw: "000001234", wantErr: true},
		{raw: "UA123", wantErr: true},
		{raw: "12a", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "12.5", wantErr: true},
		{raw: "+123", wantErr: true},
	}

	for _, tt := range tests {
		got, err := NormalizeFlightNumber(tt.raw)
		if tt.wantErr {
			if !errors.Is(err, ErrInvalidFlightNumber) || got != "" {
				t.Errorf("NormalizeFlightNumber(%q) = (%q, %v), want error %v", tt.raw, got, err, ErrInvalidFlightNumber)
			}

			continue
		}

		if err != nil || got != tt.want {
			t.Errorf("NormalizeFlightNumber(%q) = (%q, %v), want (%q, nil)", tt.raw, got, err, tt.want)
		}
	}
}

func TestMissingFlightNumberRollupKey(t *testing.T) {
	// Blank, whitespace, all zeros, and other rejected values share one bucket.
	// Empty string compares equal, so GROUP BY and the primary key collapse them.
	missing := []string{"", "   ", "0", "0000", "12a", "12345"}
	for _, raw := range missing {
		if got := WeatherRollupFlightNumber(raw); got != MissingFlightNumber {
			t.Errorf("WeatherRollupFlightNumber(%q) = %q, want empty", raw, got)
		}
	}

	if WeatherRollupFlightNumber("") != WeatherRollupFlightNumber("   ") {
		t.Fatal("missing flight numbers must compare equal")
	}

	if got := WeatherRollupFlightNumber("0123"); got != "123" {
		t.Fatalf("WeatherRollupFlightNumber(%q) = %q, want 123", "0123", got)
	}

	if WeatherRollupFlightNumber("0123") == MissingFlightNumber {
		t.Fatal("a present flight number must not use the missing bucket")
	}
}
