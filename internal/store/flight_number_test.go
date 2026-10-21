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
