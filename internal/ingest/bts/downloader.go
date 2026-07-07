package bts

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrDataNotAvailable = errors.New("bts data not available")
	ErrInvalidZip       = errors.New("response is not a valid zip file")
)

const (
	zipDatasetPrefix = "On_Time_Marketing_Carrier_On_Time_Performance_Beginning_January_2018_"
	defaultBaseURL   = "https://transtats.bts.gov/PREZIP"
)

type Downloader struct {
	baseURL    string
	httpClient *http.Client
}

func NewDownloader(baseURL string, timeout time.Duration) *Downloader {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}

	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	return &Downloader{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (d *Downloader) DownloadCSV(ctx context.Context, year, month int) (csvPath string, cleanup func(), err error) {
	url := fmt.Sprintf("%s/%s%d_%d.zip", d.baseURL, zipDatasetPrefix, year, month)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download bts zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil, fmt.Errorf("%w for %d-%d", ErrDataNotAvailable, year, month)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("download bts zip: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read bts zip: %w", err)
	}

	if len(body) < 4 || !bytes.HasPrefix(body, []byte("PK\x03\x04")) {
		return "", nil, ErrInvalidZip
	}

	tmpDir, err := os.MkdirTemp("", "bts-ingest-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	zipPath := filepath.Join(tmpDir, "data.zip")
	if err := os.WriteFile(zipPath, body, 0o600); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("write zip: %w", err)
	}

	csvPath, err = extractCSV(zipPath, tmpDir)
	if err != nil {
		cleanup()

		return "", nil, err
	}

	return csvPath, cleanup, nil
}

func extractCSV(zipPath, destDir string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	var csvName string

	for _, file := range reader.File {
		name := strings.ToLower(filepath.Base(file.Name))
		if !strings.HasSuffix(name, ".csv") {
			continue
		}

		if strings.Contains(name, "readme") {
			continue
		}

		csvName = file.Name

		break
	}

	if csvName == "" {
		return "", errors.New("csv not found in zip")
	}

	for _, file := range reader.File {
		if file.Name != csvName {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open csv in zip: %w", err)
		}

		destPath := filepath.Join(destDir, filepath.Base(file.Name))

		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			_ = rc.Close()

			return "", fmt.Errorf("create csv file: %w", err)
		}

		if _, err := io.Copy(out, io.LimitReader(rc, 1<<30)); err != nil {
			_ = rc.Close()
			_ = out.Close()

			return "", fmt.Errorf("extract csv: %w", err)
		}

		if err := out.Close(); err != nil {
			_ = rc.Close()

			return "", fmt.Errorf("close csv file: %w", err)
		}

		if err := rc.Close(); err != nil {
			return "", fmt.Errorf("close csv in zip: %w", err)
		}

		return destPath, nil
	}

	return "", errors.New("csv not found in zip")
}
