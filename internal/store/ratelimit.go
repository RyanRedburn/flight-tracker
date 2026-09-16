package store

import (
	"math"
	"time"
)

const (
	RateLimitSurfaceExternal = "external"
	RateLimitSurfaceInternal = "internal"
	RateLimitSurfaceIngest   = "ingest"

	MilliTokensPerRequest = 1000
)

type RateLimitResult struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetUnix  int64
	RetryAfter time.Duration
}

// RateLimitResultFromMilli converts shared-store milli-token state into the
// advertised header values. tokensMilli is the bucket contents after a
// successful consume, or the refilled contents when the consume was denied.
func RateLimitResultFromMilli(allowed bool, tokensMilli, rpm int, now time.Time) RateLimitResult {
	if rpm < 1 {
		rpm = 1
	}

	result := RateLimitResult{
		Allowed: allowed,
		Limit:   rpm,
	}

	if allowed {
		remaining := tokensMilli / MilliTokensPerRequest
		if remaining < 0 {
			remaining = 0
		}

		result.Remaining = remaining
		result.RetryAfter = 0
		result.ResetUnix = now.Add(timeToFull(tokensMilli, rpm)).Unix()

		return result
	}

	result.Remaining = 0
	retry := timeUntilNextToken(tokensMilli, rpm)
	result.RetryAfter = retry
	result.ResetUnix = now.Add(retry).Unix()

	return result
}

func timeUntilNextToken(tokensMilli, rpm int) time.Duration {
	need := MilliTokensPerRequest - tokensMilli
	if need <= 0 {
		return 0
	}

	return milliDuration(need, rpm)
}

func timeToFull(tokensMilli, rpm int) time.Duration {
	capacity := rpm * MilliTokensPerRequest

	need := capacity - tokensMilli
	if need <= 0 {
		return 0
	}

	return milliDuration(need, rpm)
}

func milliDuration(needMilli, rpm int) time.Duration {
	// seconds = needMilli * 60 / (rpm * 1000)
	sec := math.Ceil(float64(needMilli) * 60 / float64(rpm*MilliTokensPerRequest))
	if sec < 1 {
		sec = 1
	}

	return time.Duration(sec) * time.Second
}
