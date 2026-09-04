package postgres

import "testing"

func TestUniqueUpperCodes(t *testing.T) {
	got := uniqueUpperCodes([]string{" hnl ", "HNL", "", "aza", "ORD"})
	want := []string{"HNL", "AZA", "ORD"}

	if len(got) != len(want) {
		t.Fatalf("uniqueUpperCodes() = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueUpperCodes() = %v, want %v", got, want)
		}
	}

	if len(uniqueUpperCodes(nil)) != 0 {
		t.Fatal("uniqueUpperCodes(nil) want empty")
	}
}
