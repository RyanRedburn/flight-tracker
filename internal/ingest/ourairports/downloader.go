package ourairports

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/ingest"
	"github.com/RyanRedburn/flight-tracker/internal/store"
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
		timeout = 5 * time.Minute
	}

	return &Downloader{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (d *Downloader) DownloadCSV(ctx context.Context, dataset store.OurAirportsDataset) (path string, cleanup func(), err error) {
	filename, err := CSVFilename(dataset)
	if err != nil {
		return "", nil, err
	}

	url := fmt.Sprintf("%s/%s", d.baseURL, filename)

	return ingest.DownloadFile(ctx, d.httpClient, url)
}
