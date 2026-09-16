package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/api/handlers"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	headerAuthorization      = "Authorization"
	headerAPIKey             = "X-API-Key" //nolint:gosec // G101: HTTP header name, not a secret
	headerRateLimitLimit     = "X-RateLimit-Limit"
	headerRateLimitRemaining = "X-RateLimit-Remaining"
	headerRateLimitReset     = "X-RateLimit-Reset"
	headerRetryAfter         = "Retry-After"
	bucketKeySep             = "|"
	bucketKindKey            = "key"
	bucketKindIP             = "ip"
)

var dummyAPIKeyHash = make([]byte, 32)

type identityContextKey struct{}

type Identity struct {
	KeyID string
	Role  model.APIKeyRole
	Valid bool
}

type RateLimits struct {
	AnonRPM        int
	ConsumerRPM    int
	SubscriberRPM  int
	AdminRPM       int
	AdminIngestRPM int
	AuthFailRPM    int
}

type ProtectorConfig struct {
	Disabled          bool
	RateLimitDisabled bool
	TrustProxy        bool
	Limits            RateLimits
}

type Protector struct {
	store  store.Store
	cfg    ProtectorConfig
	shield *burstShield
	nowFn  func() time.Time
}

func NewProtector(s store.Store, cfg ProtectorConfig) *Protector {
	return &Protector{
		store:  s,
		cfg:    cfg,
		shield: newBurstShield(),
		nowFn:  func() time.Time { return time.Now().UTC() },
	}
}

func IdentityFromContext(ctx context.Context) Identity {
	id, _ := ctx.Value(identityContextKey{}).(Identity)
	return id
}

var (
	ExternalRoles = []model.APIKeyRole{
		model.APIKeyRoleConsumer,
		model.APIKeyRoleSubscriber,
		model.APIKeyRoleAdmin,
	}
	AdminOnly = []model.APIKeyRole{model.APIKeyRoleAdmin}
)

func (p *Protector) Require(allowed []model.APIKeyRole, surface string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p.cfg.Disabled {
				next.ServeHTTP(w, r)
				return
			}

			identity, err := p.resolveIdentity(r)
			if err != nil {
				handlers.WriteError(w, http.StatusInternalServerError, handlers.ErrAuthUnavailable)
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), identityContextKey{}, identity))

			if !identity.Valid {
				if !p.applyRateLimit(w, r, identity, store.RateLimitSurfaceAuthFail) {
					return
				}

				handlers.WriteError(w, http.StatusUnauthorized, handlers.ErrUnauthorized)
				return
			}

			if !p.applyRateLimit(w, r, identity, surface) {
				return
			}

			if !model.RolePermits(identity.Role, allowed) {
				handlers.WriteError(w, http.StatusForbidden, handlers.ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (p *Protector) resolveIdentity(r *http.Request) (Identity, error) {
	presented, err := extractAPIKey(r)
	if err != nil || presented == "" {
		return Identity{}, nil
	}

	return p.authenticate(r.Context(), presented)
}

func (p *Protector) applyRateLimit(w http.ResponseWriter, r *http.Request, identity Identity, surface string) bool {
	if p.cfg.RateLimitDisabled {
		return true
	}

	return p.enforceRateLimit(w, r, identity, surface)
}

func (p *Protector) authenticate(ctx context.Context, presented string) (Identity, error) {
	if presented == "" {
		return Identity{}, nil
	}

	prefix, err := model.ParseAPIKeyPrefix(presented)
	if err != nil {
		model.VerifyAPIKey(presented, dummyAPIKeyHash)
		return Identity{}, nil
	}

	key, err := p.store.LookupAPIKeyByPrefix(ctx, prefix)
	if errors.Is(err, store.ErrNotFound) {
		model.VerifyAPIKey(presented, dummyAPIKeyHash)
		return Identity{}, nil
	}

	if err != nil {
		return Identity{}, err
	}

	if key.Revoked() || !model.VerifyAPIKey(presented, key.KeyHash) {
		return Identity{}, nil
	}

	return Identity{KeyID: key.ID, Role: key.Role, Valid: true}, nil
}

func (p *Protector) enforceRateLimit(w http.ResponseWriter, r *http.Request, identity Identity, surface string) bool {
	rpm := p.limitRPM(identity, surface)
	key := rateLimitBucketKey(identity, surface, clientIP(r, p.cfg.TrustProxy))
	now := p.nowFn()

	if !p.shield.allow(key, now) {
		// Local flood shed only. Advertised X-RateLimit-* headers come from Postgres.
		writeRetryAfter(w, shieldRetryAfter)
		handlers.WriteError(w, http.StatusTooManyRequests, handlers.ErrRateLimitExceeded)

		return false
	}

	result, err := p.store.ConsumeRateLimit(r.Context(), key, rpm)
	if err != nil {
		handlers.WriteError(w, http.StatusInternalServerError, handlers.ErrRateLimitUnavailable)
		return false
	}

	if !result.Allowed {
		writeRateLimited(w, result)
		return false
	}

	setRateLimitHeaders(w, result)

	return true
}

func (p *Protector) limitRPM(identity Identity, surface string) int {
	if surface == store.RateLimitSurfaceAuthFail {
		return p.cfg.Limits.AuthFailRPM
	}

	if !identity.Valid {
		return p.cfg.Limits.AnonRPM
	}

	if surface == store.RateLimitSurfaceIngest && identity.Role == model.APIKeyRoleAdmin {
		return p.cfg.Limits.AdminIngestRPM
	}

	switch identity.Role {
	case model.APIKeyRoleAdmin:
		return p.cfg.Limits.AdminRPM
	case model.APIKeyRoleSubscriber:
		return p.cfg.Limits.SubscriberRPM
	case model.APIKeyRoleConsumer:
		return p.cfg.Limits.ConsumerRPM
	default:
		return p.cfg.Limits.AnonRPM
	}
}

func extractAPIKey(r *http.Request) (string, error) {
	headerKey := strings.TrimSpace(r.Header.Get(headerAPIKey))
	bearer, hasBearer := parseBearer(strings.TrimSpace(r.Header.Get(headerAuthorization)))

	switch {
	case hasBearer && bearer == "":
		return "", errEmptyCredential
	case hasBearer && headerKey != "" && bearer != headerKey:
		return "", errCredentialMismatch
	case hasBearer:
		return bearer, nil
	case headerKey != "":
		return headerKey, nil
	default:
		return "", nil
	}
}

func parseBearer(header string) (string, bool) {
	const word = "bearer"
	if len(header) < len(word) || !strings.EqualFold(header[:len(word)], word) {
		return "", false
	}

	rest := header[len(word):]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return "", false
	}

	return strings.TrimSpace(rest), true
}

func rateLimitBucketKey(identity Identity, surface, ip string) string {
	if identity.Valid {
		return bucketKindKey + bucketKeySep + identity.KeyID + bucketKeySep + surface
	}

	if ip == "" {
		ip = unknownClientIP
	}

	return bucketKindIP + bucketKeySep + ip + bucketKeySep + surface
}

func setRateLimitHeaders(w http.ResponseWriter, result store.RateLimitResult) {
	w.Header().Set(headerRateLimitLimit, strconv.Itoa(result.Limit))
	w.Header().Set(headerRateLimitRemaining, strconv.Itoa(result.Remaining))
	w.Header().Set(headerRateLimitReset, strconv.FormatInt(result.ResetUnix, 10))
}

func writeRateLimited(w http.ResponseWriter, result store.RateLimitResult) {
	setRateLimitHeaders(w, result)
	writeRetryAfter(w, result.RetryAfter)

	handlers.WriteError(w, http.StatusTooManyRequests, handlers.ErrRateLimitExceeded)
}

func writeRetryAfter(w http.ResponseWriter, retryAfter time.Duration) {
	sec := int(retryAfter.Seconds())
	if sec < 1 {
		sec = 1
	}

	w.Header().Set(headerRetryAfter, strconv.Itoa(sec))
}

var (
	errEmptyCredential    = errors.New("empty credential")
	errCredentialMismatch = errors.New("credential mismatch")
)
