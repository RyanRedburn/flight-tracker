package mct

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

const testIATAATL = "ATL"

func TestParseFixturePages(t *testing.T) {
	page1 := readPage(t, "testdata/airports_page1.json")
	page2 := readPage(t, "testdata/airports_page2.json")

	if page1.Total != 6 || page1.TotalPages != 2 || !page1.HasNext || page1.Page != 1 {
		t.Fatalf("page1 pagination = %+v", page1)
	}

	if page2.HasNext || page2.Page != 2 {
		t.Fatalf("page2 pagination = %+v", page2)
	}

	airports := append(append([]Airport{}, page1.Airports...), page2.Airports...)

	columns, rows, err := ToRows(airports)
	if err != nil {
		t.Fatalf("ToRows() error = %v", err)
	}

	if len(columns) != len(airportMCTColumns) {
		t.Fatalf("columns = %d, want %d", len(columns), len(airportMCTColumns))
	}

	// Invalid code skipped, duplicate ATL keeps the first row: AAA, ATL, DEN, LHR.
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}

	byCode := map[string][]string{}

	for _, row := range rows {
		if len(row) != len(columns) {
			t.Fatalf("row width %d, want %d", len(row), len(columns))
		}

		byCode[row[0]] = row
	}

	atl := byCode[testIATAATL]
	if atl == nil {
		t.Fatal("missing ATL")
	}

	if atl[1] != "Hartsfield-Jackson Atlanta International Airport" {
		t.Errorf("ATL name = %q", atl[1])
	}

	if atl[8] != "60" || atl[9] != "90" || atl[10] != "90" || atl[11] != "120" || atl[12] != "120" {
		t.Errorf("ATL mct = %v", atl[8:13])
	}

	if atl[7] != "true" || atl[13] != "2026-03-29" || atl[4] != "KATL" {
		t.Errorf("ATL identity = %v", atl)
	}

	lat, err := strconv.ParseFloat(atl[5], 64)
	if err != nil {
		t.Fatalf("ATL latitude: %v", err)
	}

	if lat != *page1.Airports[0].Latitude {
		t.Errorf("ATL latitude = %v, want %v", lat, *page1.Airports[0].Latitude)
	}

	aaa := byCode["AAA"]
	if aaa == nil || aaa[7] != "false" || aaa[8] != "35" {
		t.Fatalf("AAA = %v", aaa)
	}

	den := byCode["DEN"]
	if den == nil {
		t.Fatal("missing DEN")
	}

	if den[6] != "" {
		t.Errorf("DEN longitude = %q, want empty for out-of-range value", den[6])
	}

	if den[9] != "" || den[8] != "40" {
		t.Errorf("DEN mct domestic = %q/%q, want 40 and null international", den[8], den[9])
	}

	lhr := byCode["LHR"]
	if lhr == nil || lhr[1] != "London Heathrow" {
		t.Fatalf("LHR = %v", lhr)
	}

	if _, ok := byCode["A"]; ok {
		t.Fatal("invalid IATA was stored")
	}
}

func TestToRowsPrefersIATACode(t *testing.T) {
	_, rows, err := ToRows([]Airport{{
		Code:     "AAA",
		IATACode: "AAB",
		Name:     "Prefer iataCode",
	}})
	if err != nil {
		t.Fatalf("ToRows() error = %v", err)
	}

	if len(rows) != 1 || rows[0][0] != "AAB" {
		t.Fatalf("rows = %v, want AAB", rows)
	}
}

func TestToRowsEmpty(t *testing.T) {
	_, _, err := ToRows(nil)
	if err == nil {
		t.Fatal("ToRows() expected error")
	}
}

func TestToRowsRejectsBadDate(t *testing.T) {
	_, _, err := ToRows([]Airport{{
		IATACode:    testIATAATL,
		LastUpdated: "March 2026",
	}})
	if err == nil {
		t.Fatal("ToRows() expected date error")
	}
}

func TestParsePageRejectsBadMinutes(t *testing.T) {
	cases := []string{
		`{"data":[{"iataCode":"ATL","mct_interline":-1}],"pagination":{"page":1}}`,
		`{"data":[{"iataCode":"ATL","mct_interline":60.5}],"pagination":{"page":1}}`,
		`{"data":[{"iataCode":"ATL","mct_interline":"60"}],"pagination":{"page":1}}`,
		`{"data":[{"iataCode":"ATL","mct_interline":1e20}],"pagination":{"page":1}}`,
	}

	for _, body := range cases {
		if _, err := ParsePage([]byte(body)); err == nil {
			t.Fatalf("ParsePage(%s) expected error", body)
		}
	}
}

func TestParsePageAcceptsWholeFloatMinutes(t *testing.T) {
	page, err := ParsePage([]byte(`{"data":[{"iataCode":"ATL","mct_interline":60.0}],"pagination":{"page":1,"total":1,"total_pages":1}}`))
	if err != nil {
		t.Fatalf("ParsePage() error = %v", err)
	}

	if page.Airports[0].Interline == nil || *page.Airports[0].Interline != 60 {
		t.Fatalf("interline = %v, want 60", page.Airports[0].Interline)
	}
}

func readPage(t *testing.T, path string) Page {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}

	page, err := ParsePage(data)
	if err != nil {
		t.Fatalf("ParsePage(%s) error = %v", path, err)
	}

	return page
}

func TestImportResultJSON(t *testing.T) {
	payload, err := ImportResult{Dataset: "airport_mct", RowsImported: 4}.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded[jsonKeyDataset] != "airport_mct" {
		t.Errorf("dataset = %v", decoded[jsonKeyDataset])
	}

	if decoded[jsonKeyRows] != float64(4) {
		t.Errorf("rows_imported = %v", decoded[jsonKeyRows])
	}
}
