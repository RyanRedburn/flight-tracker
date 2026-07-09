package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

func DownloadFile(ctx context.Context, client *http.Client, url string) (path string, cleanup func(), err error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("download file: unexpected status %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "ingest-download-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	cleanup = func() { _ = os.Remove(tmpPath) }

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()

		cleanup()

		return "", nil, fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("close temp file: %w", err)
	}

	return tmpPath, cleanup, nil
}
