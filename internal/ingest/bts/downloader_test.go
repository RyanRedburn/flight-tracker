package bts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	path := filepath.Join("..", "..", "..", "test-data",
		"On_Time_Marketing_Carrier_On_Time_Performance_Beginning_January_2018_2026_4",
		"On_Time_Marketing_Carrier_On_Time_Performance_(Beginning_January_2018)_2026_4.csv",
	)

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("test csv missing at %s: %v", abs, err)
	}

	return abs
}

func minimalCSVOpener(t *testing.T) CSVOpener {
	t.Helper()

	path := writeMinimalCSV(t)

	return func(context.Context, int, int) (string, func(), error) {
		return path, func() {}, nil
	}
}

func writeMinimalCSV(t *testing.T) string {
	t.Helper()

	src, err := os.Open(testRepoCSVPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer src.Close()

	reader := csv.NewReader(src)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}

	row, err := reader.Read()
	if err != nil {
		t.Fatalf("Read row: %v", err)
	}

	path := filepath.Join(t.TempDir(), "minimal.csv")

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	writer := csv.NewWriter(out)
	if err := writer.Write(header); err != nil {
		t.Fatalf("Write header: %v", err)
	}

	if err := writer.Write(row); err != nil {
		t.Fatalf("Write row: %v", err)
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	return path
}
