package mct

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownloaderFetchesPages(t *testing.T) {
	page1, err := os.ReadFile("testdata/airports_page1.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	page2, err := os.ReadFile("testdata/airports_page2.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != airportsPath {
			t.Errorf("path = %q, want %s", r.URL.Path, airportsPath)
		}

		query := r.URL.Query()
		if query.Get("per_page") != "100" {
			t.Errorf("per_page = %q, want 100", query.Get("per_page"))
		}

		if query.Get("minimal") != "" {
			t.Errorf("minimal = %q, want omitted", query.Get("minimal"))
		}

		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}

		w.Header().Set("Content-Type", "application/json")

		switch query.Get("page") {
		case "1":
			_, _ = w.Write(page1)
		case "2":
			_, _ = w.Write(page2)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0

	airports, err := d.FetchAirports(context.Background())
	if err != nil {
		t.Fatalf("FetchAirports() error = %v", err)
	}

	if len(airports) != 6 {
		t.Fatalf("airports = %d, want 6", len(airports))
	}

	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

func TestDownloaderRetries429(t *testing.T) {
	page1, err := os.ReadFile("testdata/airports_page1.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	page2, err := os.ReadFile("testdata/airports_page2.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var calls atomic.Int32

	var slept []time.Duration

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write(page1)

			return
		}

		_, _ = w.Write(page2)
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0
	d.sleep = func(_ context.Context, wait time.Duration) error {
		slept = append(slept, wait)

		return nil
	}

	if _, err := d.FetchAirports(context.Background()); err != nil {
		t.Fatalf("FetchAirports() error = %v", err)
	}

	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}

	if len(slept) != 1 || slept[0] != 5*time.Second {
		t.Fatalf("sleeps = %v, want [5s]", slept)
	}
}

func TestDownloaderPaginationChanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"data":[{"code":"ATL","iataCode":"ATL"}],"pagination":{"has_next":true,"page":1,"per_page":100,"total":2,"total_pages":2}}`))

			return
		}

		_, _ = w.Write([]byte(`{"data":[{"code":"LHR","iataCode":"LHR"}],"pagination":{"has_next":false,"page":2,"per_page":100,"total":3,"total_pages":2}}`))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0
	d.sleep = func(context.Context, time.Duration) error { return nil }

	_, err := d.FetchAirports(context.Background())
	if err == nil {
		t.Fatal("FetchAirports() expected pagination error")
	}
}

func TestDownloaderUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0

	_, err := d.FetchAirports(context.Background())
	if err == nil {
		t.Fatal("FetchAirports() expected status error")
	}
}
