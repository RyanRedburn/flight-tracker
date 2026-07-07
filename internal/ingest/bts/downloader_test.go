package bts

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDownloader404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)

	_, cleanup, err := d.DownloadCSV(context.Background(), 2099, 1)
	if cleanup != nil {
		cleanup()
	}

	if !errors.Is(err, ErrDataNotAvailable) {
		t.Fatalf("DownloadCSV() error = %v, want ErrDataNotAvailable", err)
	}
}

func TestDownloaderInvalidZip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>redirect</html>"))
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)

	_, cleanup, err := d.DownloadCSV(context.Background(), 2026, 4)
	if cleanup != nil {
		cleanup()
	}

	if !errors.Is(err, ErrInvalidZip) {
		t.Fatalf("DownloadCSV() error = %v, want ErrInvalidZip", err)
	}
}

func TestDownloaderExtractsCSV(t *testing.T) {
	const csvBody = "Year,Quarter\n2026,2\n"

	var zipBuf bytes.Buffer

	zipWriter := zip.NewWriter(&zipBuf)

	writer, err := zipWriter.Create("On_Time_2026_4.csv")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}

	if _, err := writer.Write([]byte(csvBody)); err != nil {
		t.Fatalf("zip write: %v", err)
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBuf.Bytes())
	}))
	defer server.Close()

	d := NewDownloader(server.URL, time.Second)

	csvPath, cleanup, err := d.DownloadCSV(context.Background(), 2026, 4)
	if err != nil {
		t.Fatalf("DownloadCSV() error = %v", err)
	}
	defer cleanup()

	if !strings.HasSuffix(strings.ToLower(csvPath), ".csv") {
		t.Fatalf("csvPath = %q, want .csv suffix", csvPath)
	}

	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != csvBody {
		t.Fatalf("csv content = %q, want %q", string(data), csvBody)
	}
}

func TestDownloaderFollowsRedirect(t *testing.T) {
	const csvBody = "Year,Quarter\n2026,2\n"

	var zipBuf bytes.Buffer

	zipWriter := zip.NewWriter(&zipBuf)

	writer, err := zipWriter.Create("data.csv")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}

	if _, err := writer.Write([]byte(csvBody)); err != nil {
		t.Fatalf("zip write: %v", err)
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBuf.Bytes())
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer redirect.Close()

	d := NewDownloader(redirect.URL, time.Second)

	_, cleanup, err := d.DownloadCSV(context.Background(), 2026, 4)
	if err != nil {
		t.Fatalf("DownloadCSV() error = %v", err)
	}
	defer cleanup()
}

func testRepoCSVPath(t *testing.T) string {
	t.Helper()

	path, err := TestdataCSV()
	if err != nil {
		t.Fatalf("TestdataCSV() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("testdata csv missing at %s: %v", path, err)
	}

	return path
}

func minimalCSVOpener(t *testing.T) CSVOpener {
	t.Helper()

	path := testRepoCSVPath(t)

	return func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	}
}
