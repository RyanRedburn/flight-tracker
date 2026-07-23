package iem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	ErrRequestTooLarge = errors.New("iem request exceeds station-years limit")
	ErrEmptyStations   = errors.New("iem stations required")
)

const (
	defaultBaseURL     = "https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py"
	defaultMinInterval = time.Second
	defaultMaxAttempts = 6
)

// dataVars are IEM `data=` columns (station/valid are always returned).
var dataVars = []string{
	"tmpf", "dwpf", "relh", "drct", "sknt", "gust", "vsby",
	"skyc1", "skyc2", "skyc3", "skyl1", "skyl2", "skyl3",
	"wxcodes", "p01i", "alti", "mslp", "metar",
}

type Downloader struct {
	baseURL     string
	httpClient  *http.Client
	minInterval time.Duration
	maxAttempts int

	mu          sync.Mutex
	lastRequest time.Time

	sleep func(context.Context, time.Duration) error
	now   func() time.Time
}

func NewDownloader(baseURL string, timeout time.Duration) *Downloader {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}

	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	return &Downloader{
		baseURL: strings.TrimRight(baseURL, "?&"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		minInterval: defaultMinInterval,
		maxAttempts: defaultMaxAttempts,
		sleep:       sleepContext,
		now:         time.Now,
	}
}

func (d *Downloader) DownloadCSV(ctx context.Context, year, month int, stations []string) (csvPath string, cleanup func(), err error) {
	if len(stations) == 0 {
		return "", nil, ErrEmptyStations
	}

	reqURL, err := d.buildURL(year, month, stations)
	if err != nil {
		return "", nil, err
	}

	var lastErr error

	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		if err := d.waitTurn(ctx); err != nil {
			return "", nil, err
		}

		csvPath, cleanup, err = d.doDownload(ctx, reqURL)
		if err == nil {
			return csvPath, cleanup, nil
		}

		lastErr = err

		if errors.Is(err, ErrRequestTooLarge) {
			return "", nil, err
		}

		if !isRetryable(err) || attempt == d.maxAttempts {
			return "", nil, err
		}

		backoff := time.Duration(attempt) * 5 * time.Second
		if err := d.sleep(ctx, backoff); err != nil {
			return "", nil, err
		}
	}

	return "", nil, lastErr
}

func (d *Downloader) buildURL(year, month int, stations []string) (string, error) {
	sts, ets := monthUTCRange(year, month)

	values := url.Values{}
	values.Set("tz", "UTC")
	values.Set("format", "onlycomma")
	values.Set("missing", "empty")
	values.Set("sts", sts.Format(time.RFC3339))
	values.Set("ets", ets.Format(time.RFC3339))
	values.Add("report_type", "3")
	values.Add("report_type", "4")

	for _, station := range stations {
		values.Add("station", station)
	}

	for _, data := range dataVars {
		values.Add("data", data)
	}

	base, err := url.Parse(d.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse iem base url: %w", err)
	}

	base.RawQuery = values.Encode()

	return base.String(), nil
}

func (d *Downloader) doDownload(ctx context.Context, reqURL string) (csvPath string, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download iem csv: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnprocessableEntity:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))

		return "", nil, fmt.Errorf("%w: %s", ErrRequestTooLarge, strings.TrimSpace(string(body)))
	case resp.StatusCode == http.StatusServiceUnavailable:
		return "", nil, &retryableError{msg: fmt.Sprintf("download iem csv: status %s", resp.Status)}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return "", nil, fmt.Errorf("download iem csv: unexpected status %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "iem-asos-*.csv")
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

	headFile, err := os.Open(tmpPath)
	if err != nil {
		cleanup()

		return "", nil, fmt.Errorf("open temp file: %w", err)
	}

	head := make([]byte, 64)
	n, _ := headFile.Read(head)
	_ = headFile.Close()

	if strings.HasPrefix(string(head[:n]), "ERROR") {
		cleanup()

		msg := strings.TrimSpace(string(head[:n]))

		return "", nil, fmt.Errorf("iem service error: %s", msg)
	}

	return tmpPath, cleanup, nil
}

func (d *Downloader) waitTurn(ctx context.Context) error {
	for {
		d.mu.Lock()

		elapsed := d.now().Sub(d.lastRequest)
		if d.lastRequest.IsZero() || elapsed >= d.minInterval {
			d.lastRequest = d.now()
			d.mu.Unlock()

			return nil
		}

		wait := d.minInterval - elapsed
		d.mu.Unlock()

		if err := d.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func monthUTCRange(year, month int) (time.Time, time.Time) {
	sts := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	return sts, sts.AddDate(0, 1, 0)
}

type retryableError struct {
	msg string
}

func (e *retryableError) Error() string {
	return e.msg
}

func isRetryable(err error) bool {
	var re *retryableError

	return errors.As(err, &re)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
