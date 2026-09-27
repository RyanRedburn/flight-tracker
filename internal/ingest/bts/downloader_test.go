package bts

import (
	"archive/zip"
	"bytes"
	"context"
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

func TestCSVDestPath(t *testing.T) {
	destDir := t.TempDir()

	tests := []struct {
		name      string
		entryName string
		wantBase  string
		wantErr   bool
	}{
		{name: "simple", entryName: "On_Time_2026_4.csv", wantBase: "On_Time_2026_4.csv"},
		{name: "nested", entryName: "dir/On_Time.csv", wantBase: "On_Time.csv"},
		{name: "traversal", entryName: "nested/../../outside.csv", wantBase: "outside.csv"},
		{name: "empty", entryName: "", wantErr: true},
		{name: "dot", entryName: ".", wantErr: true},
		{name: "dotdot", entryName: "..", wantErr: true},
		{name: "nested dotdot", entryName: "foo/..", wantErr: true},
		{name: "trailing dotdot", entryName: "foo/../", wantErr: true},
	}

	cleanDir := filepath.Clean(destDir) + string(os.PathSeparator)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := csvDestPath(destDir, tt.entryName)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("csvDestPath(%q) error = nil, want error", tt.entryName)
				}

				return
			}

			if err != nil {
				t.Fatalf("csvDestPath(%q) error = %v", tt.entryName, err)
			}

			want := filepath.Join(destDir, tt.wantBase)
			if got != want {
				t.Fatalf("csvDestPath(%q) = %q, want %q", tt.entryName, got, want)
			}

			if !strings.HasPrefix(got, cleanDir) {
				t.Fatalf("csvDestPath(%q) = %q, want prefix %q", tt.entryName, got, cleanDir)
			}
		})
	}
}

func TestExtractCSVZipSlip(t *testing.T) {
	const payload = "malicious\n"

	tests := []struct {
		name      string
		entryName string
	}{
		{name: "parent", entryName: "../outside.csv"},
		{name: "nested", entryName: "nested/../../outside.csv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			destDir := filepath.Join(parent, "dest")

			if err := os.Mkdir(destDir, 0o700); err != nil {
				t.Fatalf("Mkdir() error = %v", err)
			}

			zipPath := filepath.Join(destDir, "data.zip")
			writeZipEntry(t, zipPath, tt.entryName, payload)

			outsidePath := filepath.Join(parent, "outside.csv")

			got, err := extractCSV(zipPath, destDir)
			if err != nil {
				t.Fatalf("extractCSV() error = %v", err)
			}

			if _, statErr := os.Stat(outsidePath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("outside path %s stat = %v, want not exist", outsidePath, statErr)
			}

			wantPath := filepath.Join(destDir, "outside.csv")
			if got != wantPath {
				t.Fatalf("extractCSV() path = %q, want %q", got, wantPath)
			}

			data, err := os.ReadFile(got)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			if string(data) != payload {
				t.Fatalf("csv content = %q, want %q", string(data), payload)
			}
		})
	}
}

func writeZipEntry(t *testing.T, path, entryName, body string) {
	t.Helper()

	var buf bytes.Buffer

	zipWriter := zip.NewWriter(&buf)

	writer, err := zipWriter.Create(entryName)
	if err != nil {
		t.Fatalf("zip create %q: %v", entryName, err)
	}

	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatalf("zip write %q: %v", entryName, err)
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
