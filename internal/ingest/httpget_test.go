package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownloadFile(t *testing.T) {
	const body = "id,name\n1,foo\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	path, cleanup, err := DownloadFile(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != body {
		t.Errorf("file content = %q, want %q", string(data), body)
	}
}

func TestDownloadFileUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, _, err := DownloadFile(context.Background(), server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error for non-2xx status")
	}
}
