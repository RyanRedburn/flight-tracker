package csvparse

import (
	"errors"
	"strings"
	"testing"
)

const testColName = "name"

func identityMapper(raw string) string {
	return strings.TrimSpace(raw)
}

func TestParseMissingColumn(t *testing.T) {
	csv := "id,name\n1,foo\n"

	_, _, err := Parse(strings.NewReader(csv), []string{"id", testColName, "missing"}, identityMapper)
	if err == nil {
		t.Fatal("expected error for missing column")
	}

	if !errors.Is(err, ErrMissingColumns) {
		t.Fatalf("error = %v, want ErrMissingColumns", err)
	}
}

func TestParseDuplicateHeader(t *testing.T) {
	csv := "id,id\n1,2\n"

	_, _, err := Parse(strings.NewReader(csv), []string{"id"}, identityMapper)
	if err == nil {
		t.Fatal("expected error for duplicate header")
	}

	if !strings.Contains(err.Error(), "duplicate csv column") {
		t.Fatalf("error = %v, want duplicate column message", err)
	}
}

func TestParseRowWidthMismatch(t *testing.T) {
	csv := "id,name\n1\n"

	_, _, err := Parse(strings.NewReader(csv), []string{"id", testColName}, identityMapper)
	if err == nil {
		t.Fatal("expected error for short row")
	}

	if !errors.Is(err, ErrUnexpectedCSV) {
		t.Fatalf("error = %v, want ErrUnexpectedCSV", err)
	}
}

func TestParseSuccess(t *testing.T) {
	csv := "id,name\n1,foo\n2,bar\n"

	columns, rows, err := Parse(strings.NewReader(csv), []string{"id", testColName}, identityMapper)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(columns) != 2 || columns[0] != "id" || columns[1] != testColName {
		t.Fatalf("columns = %v", columns)
	}

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	if rows[0][0] != "1" || rows[0][1] != "foo" {
		t.Errorf("rows[0] = %v", rows[0])
	}
}

func TestParseSkipsUnmappedHeaders(t *testing.T) {
	skipExtra := func(raw string) string {
		col := identityMapper(raw)
		if col == "extra" {
			return ""
		}

		return col
	}

	csv := "id,name,extra\n1,foo,ignored\n"

	columns, rows, err := Parse(strings.NewReader(csv), []string{"id", testColName}, skipExtra)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(columns) != 2 || len(rows) != 1 {
		t.Fatalf("columns = %v rows = %d", columns, len(rows))
	}
}
