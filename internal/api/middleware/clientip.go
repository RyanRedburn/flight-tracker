package middleware

import (
	"net"
	"net/http"
	"strings"
)

const (
	headerXForwardedFor = "X-Forwarded-For"
	headerXRealIP       = "X-Real-IP"
	unknownClientIP     = "unknown"
)

// clientIP returns the address used for anonymous rate-limit buckets.
//
// When trustProxy is false (default), this is the TCP peer (RemoteAddr). That
// is correct for Compose port publish and a process reached directly.
//
// When trustProxy is true, the right-most X-Forwarded-For hop is used (the
// address the immediate proxy saw), falling back to X-Real-IP, then RemoteAddr.
// Enable this only behind a single trusted reverse proxy. There is no hop-count
// stripping of extra forwarded addresses; a future edge/ingress should replace
// this if more than one proxy sits in front of the API.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := forwardedClientIP(r); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return unknownClientIP
	}

	return host
}

func forwardedClientIP(r *http.Request) string {
	if xff := r.Header.Get(headerXForwardedFor); xff != "" {
		parts := strings.Split(xff, ",")
		candidate := strings.TrimSpace(parts[len(parts)-1])

		if ip := parseIPCandidate(candidate); ip != "" {
			return ip
		}
	}

	if xri := strings.TrimSpace(r.Header.Get(headerXRealIP)); xri != "" {
		if ip := parseIPCandidate(xri); ip != "" {
			return ip
		}
	}

	return ""
}

func parseIPCandidate(s string) string {
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(s)
	if err != nil {
		return ""
	}

	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}

	return ""
}
