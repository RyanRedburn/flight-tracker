package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRouterRoutes(t *testing.T) {
	handler := newRouter(mem.New(), testLogger(), 24)

	tests := []struct {
		method     string
		path       string
		wantStatus int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/ready", http.StatusOK},
		{http.MethodGet, "/db/version", http.StatusOK},
		{http.MethodGet, "/api/v1/jobs", http.StatusOK},
		{http.MethodGet, "/api/v1/flights", http.StatusOK},
		{http.MethodGet, "/missing", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestServerShutdown(t *testing.T) {
	s := NewServer("unused", mem.New(), testLogger(), 24)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.httpServer.Serve(ln)
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve() error = %v", err)
	}
}
