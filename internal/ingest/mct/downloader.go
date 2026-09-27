package mct

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://minimumconnectiontime.com"
	defaultPerPage     = 100 // upstream caps per_page at 100
	defaultMinInterval = time.Second
	defaultMaxAttempts = 6
	defaultHTTPTimeout = 2 * time.Minute
	maxBackoff         = 30 * time.Second
	maxPages           = 500
	maxPageBytes       = 32 << 20
	airportsPath       = "/api/airports"
	userAgent          = "flight-tracker"
)

// Downloader pages GET {base}/api/airports. minimal=true is not sent: that
// view omits the MCT minute fields. Requests are spaced and retried so a
// full catalog refresh stays within the public rate limit.
type Downloader struct {
	baseURL     string
	perPage     int
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
		timeout = defaultHTTPTimeout
	}

	return &Downloader{
		baseURL: strings.TrimRight(baseURL, "/"),
		perPage: defaultPerPage,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		minInterval: defaultMinInterval,
		maxAttempts: defaultMaxAttempts,
		sleep:       sleepContext,
		now:         time.Now,
	}
}

// FetchAirports downloads every page. A partial or empty catalog returns an
// error and does not imply the caller should replace stored rows.
func (d *Downloader) FetchAirports(ctx context.Context) ([]Airport, error) {
	first, err := d.fetchPage(ctx, 1)
	if err != nil {
		return nil, err
	}

	if err := validateCatalogSize(first); err != nil {
		return nil, err
	}

	if err := validatePage(1, first, first.Total, first.TotalPages); err != nil {
		return nil, err
	}

	all := make([]Airport, 0, first.Total)
	all = append(all, first.Airports...)

	for page := 2; page <= first.TotalPages; page++ {
		next, err := d.fetchPage(ctx, page)
		if err != nil {
			return nil, err
		}

		if err := validatePage(page, next, first.Total, first.TotalPages); err != nil {
			return nil, err
		}

		all = append(all, next.Airports...)
	}

	if len(all) != first.Total {
		return nil, fmt.Errorf("mct airports: fetched %d rows, pagination total %d", len(all), first.Total)
	}

	return all, nil
}

func (d *Downloader) fetchPage(ctx context.Context, page int) (Page, error) {
	body, err := d.getPage(ctx, page)
	if err != nil {
		return Page{}, err
	}

	parsed, err := ParsePage(body)
	if err != nil {
		return Page{}, err
	}

	return parsed, nil
}

func (d *Downloader) getPage(ctx context.Context, page int) ([]byte, error) {
	reqURL, err := d.pageURL(page)
	if err != nil {
		return nil, err
	}

	var lastErr error

	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		if err := d.waitTurn(ctx); err != nil {
			return nil, err
		}

		body, err := d.doGet(ctx, reqURL)
		if err == nil {
			return body, nil
		}

		lastErr = err

		if !isRetryable(err) || attempt == d.maxAttempts {
			return nil, err
		}

		wait := retryWait(err, attempt)
		if err := d.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func (d *Downloader) pageURL(page int) (string, error) {
	perPage := d.perPage
	if perPage <= 0 {
		perPage = defaultPerPage
	}

	base, err := url.Parse(d.baseURL + airportsPath)
	if err != nil {
		return "", fmt.Errorf("parse mct base url: %w", err)
	}

	query := base.Query()
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))
	base.RawQuery = query.Encode()

	return base.String(), nil
}

func (d *Downloader) doGet(ctx context.Context, reqURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build mct request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}

		return nil, &retryableError{msg: "download mct airports: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes+1))
	if err != nil {
		return nil, &retryableError{msg: "download mct airports: " + err.Error()}
	}

	if len(body) > maxPageBytes {
		return nil, fmt.Errorf("download mct airports: page exceeds %d bytes", maxPageBytes)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return nil, &retryableError{
			msg:  "download mct airports: status " + resp.Status,
			wait: retryAfter(resp.Header),
		}
	default:
		return nil, fmt.Errorf("download mct airports: unexpected status %s", resp.Status)
	}
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

func validateCatalogSize(page Page) error {
	if page.Total < 1 || page.TotalPages < 1 {
		return errors.New("mct airports: empty catalog")
	}

	if page.TotalPages > maxPages {
		return fmt.Errorf("mct airports: total_pages %d exceeds cap %d", page.TotalPages, maxPages)
	}

	return nil
}

func validatePage(requested int, page Page, expectedTotal, expectedPages int) error {
	if page.Page != requested {
		return fmt.Errorf("mct airports: requested page %d, got %d", requested, page.Page)
	}

	if page.Total != expectedTotal || page.TotalPages != expectedPages {
		return errors.New("mct airports: pagination changed during fetch")
	}

	wantNext := requested < expectedPages
	if page.HasNext != wantNext {
		return fmt.Errorf("mct airports: page %d has_next=%t", requested, page.HasNext)
	}

	return nil
}

func retryWait(err error, attempt int) time.Duration {
	wait := time.Duration(attempt) * time.Second

	var retry *retryableError
	if errors.As(err, &retry) && retry.wait > wait {
		wait = retry.wait
	}

	if wait > maxBackoff {
		return maxBackoff
	}

	return wait
}

func retryAfter(header http.Header) time.Duration {
	raw := strings.TrimSpace(header.Get("Retry-After"))
	if raw == "" {
		return 0
	}

	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return 0
	}

	return time.Duration(secs) * time.Second
}

type retryableError struct {
	msg  string
	wait time.Duration
}

func (e *retryableError) Error() string {
	return e.msg
}

func isRetryable(err error) bool {
	var retry *retryableError

	return errors.As(err, &retry)
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
