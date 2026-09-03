package postgres

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildCopySQL(t *testing.T) {
	got, err := buildCopySQL("weather_observations", []string{testColStation, testColValid})
	if err != nil {
		t.Fatalf("buildCopySQL() error = %v", err)
	}

	want := `COPY "weather_observations" ("station", "valid") FROM STDIN WITH (FORMAT csv)`
	if got != want {
		t.Errorf("buildCopySQL() = %q, want %q", got, want)
	}
}

func TestEncodeCopyCSVEmptyField(t *testing.T) {
	var buf bytes.Buffer

	err := encodeCopyCSV(&buf, []string{testColStation, testColGust}, [][]string{
		{testAirportORD, ""},
	})
	if err != nil {
		t.Fatalf("encodeCopyCSV() error = %v", err)
	}

	got := buf.String()
	if got != testAirportORD+",\n" {
		t.Errorf("encodeCopyCSV() = %q, want empty gust field", got)
	}
}

func TestEncodeCopyCSVQuotesField(t *testing.T) {
	var buf bytes.Buffer

	err := encodeCopyCSV(&buf, []string{testColStation, testColMetar}, [][]string{
		{testAirportORD, `KORD 5" SN`},
	})
	if err != nil {
		t.Fatalf("encodeCopyCSV() error = %v", err)
	}

	if !strings.Contains(buf.String(), `""`) {
		t.Errorf("encodeCopyCSV() = %q, want escaped quote", buf.String())
	}
}

func TestEncodeCopyCSVWidthMismatch(t *testing.T) {
	err := encodeCopyCSV(&bytes.Buffer{}, []string{testColStation, testColGust}, [][]string{
		{testAirportORD},
	})
	if err == nil {
		t.Fatal("encodeCopyCSV() expected width error")
	}
}
