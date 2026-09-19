package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testProxyRemoteAddr = "10.0.0.2:8080"
	testProxyHost       = "10.0.0.2"
)

func TestClientIPRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:12345"

	got := clientIP(req, false)
	if got != "203.0.113.10" {
		t.Fatalf("clientIP() = %q, want 203.0.113.10", got)
	}
}

func TestClientIPIgnoresForwardedWhenUntrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testProxyRemoteAddr
	req.Header.Set("X-Forwarded-For", "198.51.100.4, 10.0.0.2")

	got := clientIP(req, false)
	if got != testProxyHost {
		t.Fatalf("clientIP(untrusted) = %q, want %s", got, testProxyHost)
	}
}

func TestClientIPTrustedProxyUsesRightmostXFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testProxyRemoteAddr
	req.Header.Set("X-Forwarded-For", "198.51.100.4, 203.0.113.9")

	got := clientIP(req, true)
	if got != "203.0.113.9" {
		t.Fatalf("clientIP(trusted) = %q, want 203.0.113.9", got)
	}
}

func TestClientIPTrustedProxyRealIPFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testProxyRemoteAddr
	req.Header.Set("X-Real-IP", "203.0.113.20")

	got := clientIP(req, true)
	if got != "203.0.113.20" {
		t.Fatalf("clientIP(real-ip) = %q, want 203.0.113.20", got)
	}
}

func TestClientIPUnknown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ""

	got := clientIP(req, false)
	if got != unknownClientIP {
		t.Fatalf("clientIP(empty) = %q, want %q", got, unknownClientIP)
	}
}
