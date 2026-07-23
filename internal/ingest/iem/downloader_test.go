package iem

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildURL(t *testing.T) {
	d := NewDownloader("https://example.test/asos.py", time.Second)

	raw, err := d.buildURL(2024, 1, []string{"ORD", "JFK"})
	if err != nil {
		t.Fatalf("buildURL() error = %v", err)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	q := parsed.Query()
	if q.Get("tz") != "UTC" || q.Get("format") != "onlycomma" || q.Get("missing") != "empty" {
		t.Fatalf("query = %v, want tz/format/missing defaults", q)
	}

	if q.Get("sts") != "2024-01-01T00:00:00Z" || q.Get("ets") != "2024-02-01T00:00:00Z" {
		t.Fatalf("sts/ets = %q/%q", q.Get("sts"), q.Get("ets"))
	}

	if got := q["station"]; len(got) != 2 || got[0] != "ORD" || got[1] != "JFK" {
		t.Fatalf("station = %v, want [ORD JFK]", got)
	}

	if got := q["report_type"]; len(got) != 2 || got[0] != "3" || got[1] != "4" {
		t.Fatalf("report_type = %v, want [3 4]", got)
	}

	if got := q["data"]; len(got) != len(dataVars) {
		t.Fatalf("data count = %d, want %d", len(got), len(dataVars))
	}
}

func TestDownloaderEmptyStations(t *testing.T) {
	d := NewDownloader("https://example.test/asos.py", time.Second)

	_, cleanup, err := d.DownloadCSV(context.Background(), 2024, 1, nil)
	if cleanup != nil {
		cleanup()
	}

	if !errors.Is(err, ErrEmptyStations) {
		t.Fatalf("DownloadCSV() error = %v, want ErrEmptyStations", err)
	}
}

func TestDownloaderWritesCSV(t *testing.T) {
	const csvBody = "station,valid,tmpf\nORD,2024-01-01 00:51,32.00\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("station") != "ORD" {
			t.Errorf("station = %q, want ORD", r.URL.Query().Get("station"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(csvBody))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0

	csvPath, cleanup, err := d.DownloadCSV(context.Background(), 2024, 1, []string{"ORD"})
	if err != nil {
		t.Fatalf("DownloadCSV() error = %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != csvBody {
		t.Fatalf("csv content = %q, want %q", string(data), csvBody)
	}
}

func TestDownloader422(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("reduce request size"))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0

	_, cleanup, err := d.DownloadCSV(context.Background(), 2024, 1, []string{"ORD"})
	if cleanup != nil {
		cleanup()
	}

	if !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("DownloadCSV() error = %v, want ErrRequestTooLarge", err)
	}
}

func TestDownloaderRetries503(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("station,valid,tmpf\nORD,2024-01-01 00:51,32.00\n"))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0
	d.sleep = func(context.Context, time.Duration) error { return nil }

	_, cleanup, err := d.DownloadCSV(context.Background(), 2024, 1, []string{"ORD"})
	if err != nil {
		t.Fatalf("DownloadCSV() error = %v", err)
	}
	defer cleanup()

	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestDownloaderERRORBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ERROR: boom"))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = 0

	_, cleanup, err := d.DownloadCSV(context.Background(), 2024, 1, []string{"ORD"})
	if cleanup != nil {
		cleanup()
	}

	if err == nil || !strings.Contains(err.Error(), "iem service error") {
		t.Fatalf("DownloadCSV() error = %v, want iem service error", err)
	}
}

func TestDownloaderThrottles(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("station,valid\nORD,2024-01-01 00:51\n"))
	}))
	defer server.Close()

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var slept time.Duration

	d := NewDownloader(server.URL, time.Second)
	d.minInterval = time.Second
	d.now = func() time.Time { return now }
	d.sleep = func(_ context.Context, wait time.Duration) error {
		slept += wait
		now = now.Add(wait)

		return nil
	}

	for i := 0; i < 2; i++ {
		_, cleanup, err := d.DownloadCSV(context.Background(), 2024, 1, []string{"ORD"})
		if err != nil {
			t.Fatalf("DownloadCSV() error = %v", err)
		}
		cleanup()
	}

	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}

	if slept < time.Second {
		t.Fatalf("slept = %v, want at least 1s throttle", slept)
	}
}

func TestMonthUTCRange(t *testing.T) {
	sts, ets := monthUTCRange(2024, 12)
	if sts.Format(time.RFC3339) != "2024-12-01T00:00:00Z" {
		t.Fatalf("sts = %s", sts.Format(time.RFC3339))
	}

	if ets.Format(time.RFC3339) != "2025-01-01T00:00:00Z" {
		t.Fatalf("ets = %s", ets.Format(time.RFC3339))
	}
}
