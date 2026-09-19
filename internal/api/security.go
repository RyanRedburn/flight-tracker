package api

import "github.com/RyanRedburn/flight-tracker/internal/api/middleware"

// Security controls API authentication and shared rate limiting.
// Disabled is an explicit fail-open for local/dev; production must leave it false.
type Security struct {
	Disabled          bool
	RateLimitDisabled bool
	TrustProxy        bool
	Limits            middleware.RateLimits
}
