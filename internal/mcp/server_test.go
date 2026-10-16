package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestNewHandlerRejectsForeignOrigin(t *testing.T) {
	srv := httptest.NewServer(NewHandler(&storetest.Stub{}, testLogger()))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	req.Header.Set("Origin", "https://evil.example")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
