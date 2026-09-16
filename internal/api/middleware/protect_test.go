package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/api/handlers"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestExtractAPIKey(t *testing.T) {
	secret, _, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	tests := []struct {
		name    string
		headers map[string]string
		want    string
		wantErr bool
	}{
		{name: "missing", want: ""},
		{name: "bearer", headers: map[string]string{headerAuthorization: "Bearer " + secret}, want: secret},
		{name: "bearer case", headers: map[string]string{headerAuthorization: "bearer " + secret}, want: secret},
		{name: "x-api-key", headers: map[string]string{headerAPIKey: secret}, want: secret},
		{name: "both match", headers: map[string]string{headerAuthorization: "Bearer " + secret, headerAPIKey: secret}, want: secret},
		{name: "mismatch", headers: map[string]string{headerAuthorization: "Bearer " + secret, headerAPIKey: secret + "x"}, wantErr: true},
		{name: "empty bearer", headers: map[string]string{headerAuthorization: "Bearer "}, wantErr: true},
		{name: "basic ignored", headers: map[string]string{headerAuthorization: "Basic abc"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got, err := extractAPIKey(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("extractAPIKey() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("extractAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBurstShieldAllowsThenDenies(t *testing.T) {
	s := newBurstShield()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < shieldCapacity; i++ {
		if !s.allow("key|1|external", now) {
			t.Fatalf("request %d denied, want allowed", i+1)
		}
	}

	if s.allow("key|1|external", now) {
		t.Fatal("expected shield deny after capacity")
	}

	if !s.allow("key|1|external", now.Add(time.Second)) {
		t.Fatal("expected allow after refill")
	}
}

func TestRateLimitBucketKey(t *testing.T) {
	authed := Identity{KeyID: "abc", Valid: true}

	got := rateLimitBucketKey(authed, "external", "203.0.113.1")
	if got != "key|abc|external" {
		t.Fatalf("authed bucket = %q", got)
	}

	anon := rateLimitBucketKey(Identity{}, "external", "203.0.113.1")
	if anon != "ip|203.0.113.1|external" {
		t.Fatalf("anon bucket = %q", anon)
	}

	authFail := rateLimitBucketKey(Identity{}, store.RateLimitSurfaceAuthFail, "203.0.113.1")
	if authFail != "ip|203.0.113.1|auth_fail" {
		t.Fatalf("auth_fail bucket = %q", authFail)
	}
}

func TestProtectorRequire(t *testing.T) {
	plaintext, prefix, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	active := &model.APIKey{
		ID:        "key-1",
		Prefix:    prefix,
		KeyHash:   model.HashAPIKey(plaintext),
		Role:      model.APIKeyRoleConsumer,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	revokedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	revoked := *active
	revoked.RevokedAt = &revokedAt

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	limits := RateLimits{AnonRPM: 60, ConsumerRPM: 120, SubscriberRPM: 120, AdminRPM: 300, AdminIngestRPM: 10, AuthFailRPM: 30}

	tests := []struct {
		name       string
		disabled   bool
		rlDisabled bool
		role       model.APIKeyRole
		key        *model.APIKey
		header     string
		headerName string
		allowed    []model.APIKeyRole
		consume    store.RateLimitResult
		consumeErr error
		wantStatus int
		wantRetry  bool
	}{
		{
			name:       "disabled skips auth",
			disabled:   true,
			allowed:    ExternalRoles,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "missing key",
			allowed:    ExternalRoles,
			consume:    store.RateLimitResult{Allowed: true, Limit: 30, Remaining: 29, ResetUnix: 1},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "bearer consumer",
			key:        active,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			allowed:    ExternalRoles,
			consume:    store.RateLimitResult{Allowed: true, Limit: 120, Remaining: 119, ResetUnix: 1},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "x-api-key consumer",
			key:        active,
			header:     plaintext,
			headerName: headerAPIKey,
			allowed:    ExternalRoles,
			consume:    store.RateLimitResult{Allowed: true, Limit: 120, Remaining: 119, ResetUnix: 1},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "consumer forbidden on admin",
			key:        active,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			allowed:    AdminOnly,
			consume:    store.RateLimitResult{Allowed: true, Limit: 120, Remaining: 119, ResetUnix: 1},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "revoked",
			key:        &revoked,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			allowed:    ExternalRoles,
			consume:    store.RateLimitResult{Allowed: true, Limit: 30, Remaining: 29, ResetUnix: 1},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rate limited",
			key:        active,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			allowed:    ExternalRoles,
			consume:    store.RateLimitResult{Allowed: false, Limit: 120, Remaining: 0, ResetUnix: 99, RetryAfter: 5 * time.Second},
			wantStatus: http.StatusTooManyRequests,
			wantRetry:  true,
		},
		{
			name:       "rate limit disabled still auths",
			rlDisabled: true,
			key:        active,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			allowed:    ExternalRoles,
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &storetest.Stub{
				LookupAPIKeyByPrefixFn: func(_ context.Context, gotPrefix string) (*model.APIKey, error) {
					if tt.key != nil && gotPrefix == tt.key.Prefix {
						copied := *tt.key
						copied.KeyHash = append([]byte(nil), tt.key.KeyHash...)

						return &copied, nil
					}

					return nil, store.ErrNotFound
				},
				ConsumeRateLimitFn: func(context.Context, string, int) (store.RateLimitResult, error) {
					if tt.consumeErr != nil {
						return store.RateLimitResult{}, tt.consumeErr
					}

					return tt.consume, nil
				},
			}

			p := NewProtector(stub, ProtectorConfig{
				Disabled:          tt.disabled,
				RateLimitDisabled: tt.rlDisabled,
				Limits:            limits,
			})
			h := p.Require(tt.allowed, store.RateLimitSurfaceExternal)(next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats", nil)
			req.RemoteAddr = "203.0.113.8:9"

			if tt.headerName != "" {
				req.Header.Set(tt.headerName, tt.header)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantStatus == http.StatusTooManyRequests || tt.wantStatus == http.StatusNoContent && !tt.disabled && !tt.rlDisabled && tt.key != nil {
				if tt.wantStatus != http.StatusUnauthorized && tt.wantStatus != http.StatusForbidden && rec.Code == http.StatusNoContent {
					if rec.Header().Get(headerRateLimitLimit) == "" {
						t.Fatal("missing X-RateLimit-Limit on allowed response")
					}
				}
			}

			if tt.wantRetry {
				if rec.Header().Get(headerRetryAfter) != "5" {
					t.Fatalf("Retry-After = %q, want 5", rec.Header().Get(headerRetryAfter))
				}

				if rec.Header().Get(headerRateLimitRemaining) != "0" {
					t.Fatalf("Remaining = %q, want 0", rec.Header().Get(headerRateLimitRemaining))
				}

				var body handlers.ErrorResponse
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if body.Error != handlers.ErrRateLimitExceeded {
					t.Fatalf("error = %q", body.Error)
				}
			}
		})
	}
}

func TestProtectorShieldDenyOmitsSharedHeaders(t *testing.T) {
	plaintext, prefix, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	key := &model.APIKey{
		ID:        "key-1",
		Prefix:    prefix,
		KeyHash:   model.HashAPIKey(plaintext),
		Role:      model.APIKeyRoleConsumer,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	p := NewProtector(&storetest.Stub{
		LookupAPIKeyByPrefixFn: func(_ context.Context, gotPrefix string) (*model.APIKey, error) {
			if gotPrefix == key.Prefix {
				copied := *key
				copied.KeyHash = append([]byte(nil), key.KeyHash...)

				return &copied, nil
			}

			return nil, store.ErrNotFound
		},
		ConsumeRateLimitFn: func(context.Context, string, int) (store.RateLimitResult, error) {
			t.Fatal("ConsumeRateLimit should not run on shield deny")

			return store.RateLimitResult{}, nil
		},
	}, ProtectorConfig{
		Limits: RateLimits{AnonRPM: 60, ConsumerRPM: 120, SubscriberRPM: 120, AdminRPM: 300, AdminIngestRPM: 10, AuthFailRPM: 30},
	})

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p.nowFn = func() time.Time { return now }

	identity := Identity{KeyID: key.ID, Valid: true}
	bucket := rateLimitBucketKey(identity, store.RateLimitSurfaceExternal, "203.0.113.8")

	for i := 0; i < shieldCapacity; i++ {
		if !p.shield.allow(bucket, now) {
			t.Fatalf("prefill request %d denied, want allowed", i+1)
		}
	}

	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run after shield deny")
	})

	h := p.Require(ExternalRoles, store.RateLimitSurfaceExternal)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats", nil)
	req.RemoteAddr = "203.0.113.8:9"
	req.Header.Set(headerAuthorization, "Bearer "+plaintext)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}

	if rec.Header().Get(headerRetryAfter) != "1" {
		t.Fatalf("Retry-After = %q, want 1", rec.Header().Get(headerRetryAfter))
	}

	for _, name := range []string{headerRateLimitLimit, headerRateLimitRemaining, headerRateLimitReset} {
		if got := rec.Header().Get(name); got != "" {
			t.Errorf("%s = %q, want absent", name, got)
		}
	}

	var body handlers.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Error != handlers.ErrRateLimitExceeded {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestProtectorRateLimitBuckets(t *testing.T) {
	plaintext, prefix, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	unknown, _, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey unknown: %v", err)
	}

	active := &model.APIKey{
		ID:        "key-1",
		Prefix:    prefix,
		KeyHash:   model.HashAPIKey(plaintext),
		Role:      model.APIKeyRoleConsumer,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	revokedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	revoked := *active
	revoked.RevokedAt = &revokedAt

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	limits := RateLimits{AnonRPM: 60, ConsumerRPM: 120, SubscriberRPM: 120, AdminRPM: 300, AdminIngestRPM: 10, AuthFailRPM: 30}
	clientIP := "203.0.113.8"
	authFailBucket := bucketKindIP + bucketKeySep + clientIP + bucketKeySep + store.RateLimitSurfaceAuthFail
	keyBucket := bucketKindKey + bucketKeySep + active.ID + bucketKeySep + store.RateLimitSurfaceExternal
	allowedAuthFail := store.RateLimitResult{Allowed: true, Limit: 30, Remaining: 29, ResetUnix: 1}

	tests := []struct {
		name       string
		key        *model.APIKey
		header     string
		headerName string
		consume    store.RateLimitResult
		wantBucket string
		wantRPM    int
		wantStatus int
		wantRetry  string
		wantQuota  bool
	}{
		{
			name:       "missing key uses auth_fail not anon surface",
			consume:    allowedAuthFail,
			wantBucket: authFailBucket,
			wantRPM:    30,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed key uses auth_fail",
			header:     "not-a-key",
			headerName: headerAPIKey,
			consume:    allowedAuthFail,
			wantBucket: authFailBucket,
			wantRPM:    30,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown key uses auth_fail",
			header:     "Bearer " + unknown,
			headerName: headerAuthorization,
			consume:    allowedAuthFail,
			wantBucket: authFailBucket,
			wantRPM:    30,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "revoked key uses auth_fail",
			key:        &revoked,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			consume:    allowedAuthFail,
			wantBucket: authFailBucket,
			wantRPM:    30,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty bearer uses auth_fail",
			header:     "Bearer ",
			headerName: headerAuthorization,
			consume:    allowedAuthFail,
			wantBucket: authFailBucket,
			wantRPM:    30,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid key uses key surface bucket",
			key:        active,
			header:     "Bearer " + plaintext,
			headerName: headerAuthorization,
			consume:    store.RateLimitResult{Allowed: true, Limit: 120, Remaining: 119, ResetUnix: 1},
			wantBucket: keyBucket,
			wantRPM:    120,
			wantStatus: http.StatusNoContent,
			wantQuota:  true,
		},
		{
			name:       "auth_fail exhausted returns postgres 429 headers",
			consume:    store.RateLimitResult{Allowed: false, Limit: 30, Remaining: 0, ResetUnix: 99, RetryAfter: 4 * time.Second},
			wantBucket: authFailBucket,
			wantRPM:    30,
			wantStatus: http.StatusTooManyRequests,
			wantRetry:  "4",
			wantQuota:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBucket string
			var gotRPM int
			var consumeCalls int

			stub := &storetest.Stub{
				LookupAPIKeyByPrefixFn: func(_ context.Context, gotPrefix string) (*model.APIKey, error) {
					if tt.key != nil && gotPrefix == tt.key.Prefix {
						copied := *tt.key
						copied.KeyHash = append([]byte(nil), tt.key.KeyHash...)

						return &copied, nil
					}

					return nil, store.ErrNotFound
				},
				ConsumeRateLimitFn: func(_ context.Context, bucket string, rpm int) (store.RateLimitResult, error) {
					consumeCalls++
					gotBucket = bucket
					gotRPM = rpm

					return tt.consume, nil
				},
			}

			p := NewProtector(stub, ProtectorConfig{Limits: limits})
			h := p.Require(ExternalRoles, store.RateLimitSurfaceExternal)(next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats", nil)
			req.RemoteAddr = clientIP + ":9"

			if tt.headerName != "" {
				req.Header.Set(tt.headerName, tt.header)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if consumeCalls != 1 {
				t.Fatalf("ConsumeRateLimit calls = %d, want 1", consumeCalls)
			}

			if gotBucket != tt.wantBucket {
				t.Errorf("bucket = %q, want %q", gotBucket, tt.wantBucket)
			}

			if gotRPM != tt.wantRPM {
				t.Errorf("rpm = %d, want %d", gotRPM, tt.wantRPM)
			}

			if tt.wantQuota {
				if rec.Header().Get(headerRateLimitLimit) != strconv.Itoa(tt.consume.Limit) {
					t.Errorf("X-RateLimit-Limit = %q, want %d", rec.Header().Get(headerRateLimitLimit), tt.consume.Limit)
				}

				if rec.Header().Get(headerRateLimitRemaining) != strconv.Itoa(tt.consume.Remaining) {
					t.Errorf("X-RateLimit-Remaining = %q, want %d", rec.Header().Get(headerRateLimitRemaining), tt.consume.Remaining)
				}

				if rec.Header().Get(headerRateLimitReset) != strconv.FormatInt(tt.consume.ResetUnix, 10) {
					t.Errorf("X-RateLimit-Reset = %q, want %d", rec.Header().Get(headerRateLimitReset), tt.consume.ResetUnix)
				}
			}

			if tt.wantRetry != "" && rec.Header().Get(headerRetryAfter) != tt.wantRetry {
				t.Errorf("Retry-After = %q, want %q", rec.Header().Get(headerRetryAfter), tt.wantRetry)
			}
		})
	}
}

func TestProtectorLimitRPM(t *testing.T) {
	p := NewProtector(&storetest.Stub{}, ProtectorConfig{
		Limits: RateLimits{
			AnonRPM:        60,
			ConsumerRPM:    120,
			SubscriberRPM:  121,
			AdminRPM:       300,
			AdminIngestRPM: 10,
			AuthFailRPM:    30,
		},
	})

	if got := p.limitRPM(Identity{}, store.RateLimitSurfaceAuthFail); got != 30 {
		t.Errorf("auth_fail = %d, want 30", got)
	}

	if got := p.limitRPM(Identity{}, store.RateLimitSurfaceExternal); got != 60 {
		t.Errorf("anon = %d, want 60", got)
	}

	if got := p.limitRPM(Identity{Valid: true, Role: model.APIKeyRoleConsumer}, store.RateLimitSurfaceExternal); got != 120 {
		t.Errorf("consumer = %d, want 120", got)
	}

	if got := p.limitRPM(Identity{Valid: true, Role: model.APIKeyRoleSubscriber}, store.RateLimitSurfaceExternal); got != 121 {
		t.Errorf("subscriber = %d, want 121", got)
	}

	if got := p.limitRPM(Identity{Valid: true, Role: model.APIKeyRoleAdmin}, store.RateLimitSurfaceInternal); got != 300 {
		t.Errorf("admin internal = %d, want 300", got)
	}

	if got := p.limitRPM(Identity{Valid: true, Role: model.APIKeyRoleAdmin}, store.RateLimitSurfaceIngest); got != 10 {
		t.Errorf("admin ingest = %d, want 10", got)
	}
}
