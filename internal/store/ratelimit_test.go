package store

import (
	"testing"
	"time"
)

func TestRateLimitResultFromMilliAllowed(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	result := RateLimitResultFromMilli(true, 119*MilliTokensPerRequest, 120, now)

	if !result.Allowed {
		t.Fatal("Allowed = false, want true")
	}

	if result.Limit != 120 {
		t.Errorf("Limit = %d, want 120", result.Limit)
	}

	if result.Remaining != 119 {
		t.Errorf("Remaining = %d, want 119", result.Remaining)
	}

	if result.RetryAfter != 0 {
		t.Errorf("RetryAfter = %s, want 0", result.RetryAfter)
	}

	if result.ResetUnix <= now.Unix() {
		t.Errorf("ResetUnix = %d, want after %d", result.ResetUnix, now.Unix())
	}
}

func TestRateLimitResultFromMilliDenied(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	result := RateLimitResultFromMilli(false, 0, 10, now)

	if result.Allowed {
		t.Fatal("Allowed = true, want false")
	}

	if result.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", result.Remaining)
	}

	if result.RetryAfter < time.Second {
		t.Errorf("RetryAfter = %s, want at least 1s", result.RetryAfter)
	}

	if result.ResetUnix != now.Add(result.RetryAfter).Unix() {
		t.Errorf("ResetUnix = %d, want %d", result.ResetUnix, now.Add(result.RetryAfter).Unix())
	}
}

func TestTimeUntilNextTokenFull(t *testing.T) {
	if d := timeUntilNextToken(MilliTokensPerRequest, 60); d != 0 {
		t.Errorf("timeUntilNextToken(full) = %s, want 0", d)
	}

	if d := timeToFull(60*MilliTokensPerRequest, 60); d != 0 {
		t.Errorf("timeToFull(at capacity) = %s, want 0", d)
	}
}
