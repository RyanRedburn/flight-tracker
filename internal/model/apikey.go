package model

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type APIKeyRole string

const (
	APIKeyRoleConsumer   APIKeyRole = "consumer"
	APIKeyRoleSubscriber APIKeyRole = "subscriber"
	APIKeyRoleAdmin      APIKeyRole = "admin"
)

const (
	APIKeyTokenPrefix = "ftk_"
	apiKeyPrefixBytes = 4
	apiKeySecretBytes = 16
	apiKeyPrefixLen   = apiKeyPrefixBytes * 2
	apiKeySecretLen   = apiKeySecretBytes * 2
)

var (
	ErrInvalidAPIKeyFormat = errors.New("invalid API key format")
	ErrInvalidAPIKeyRole   = errors.New("role must be consumer, subscriber, or admin")
	errAPIKeyNameTooLong   = errors.New("name must be at most 100 characters")
)

// APIKey is the persisted API credential metadata. KeyHash is never serialized.
type APIKey struct {
	ID        string     `json:"id"`
	Prefix    string     `json:"prefix"`
	KeyHash   []byte     `json:"-"`
	Role      APIKeyRole `json:"role"`
	Name      string     `json:"name,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func (k APIKey) Revoked() bool {
	return k.RevokedAt != nil
}

func (r APIKeyRole) Valid() bool {
	switch r {
	case APIKeyRoleConsumer, APIKeyRoleSubscriber, APIKeyRoleAdmin:
		return true
	default:
		return false
	}
}

// RolePermits reports whether have is in allowed. Admin is not implied; callers
// must include APIKeyRoleAdmin when operators should have access.
func RolePermits(have APIKeyRole, allowed []APIKeyRole) bool {
	for _, role := range allowed {
		if have == role {
			return true
		}
	}

	return false
}

type CreateAPIKeyRequest struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

func (r CreateAPIKeyRequest) Validate() error {
	if !APIKeyRole(r.Role).Valid() {
		return ErrInvalidAPIKeyRole
	}

	if len(strings.TrimSpace(r.Name)) > 100 {
		return errAPIKeyNameTooLong
	}

	return nil
}

// GenerateAPIKey returns a new plaintext key and its lookup prefix.
// Format: ftk_<8 hex prefix>_<32 hex secret>. Only the SHA-256 hash is stored.
func GenerateAPIKey() (string, string, error) {
	prefixBytes := make([]byte, apiKeyPrefixBytes)
	secretBytes := make([]byte, apiKeySecretBytes)

	if _, err := rand.Read(prefixBytes); err != nil {
		return "", "", err
	}

	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}

	prefix := hex.EncodeToString(prefixBytes)
	secret := hex.EncodeToString(secretBytes)

	return APIKeyTokenPrefix + prefix + "_" + secret, prefix, nil
}

func HashAPIKey(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

func ParseAPIKeyPrefix(plaintext string) (string, error) {
	if !validAPIKeyFormat(plaintext) {
		return "", ErrInvalidAPIKeyFormat
	}

	return plaintext[len(APIKeyTokenPrefix) : len(APIKeyTokenPrefix)+apiKeyPrefixLen], nil
}

func VerifyAPIKey(plaintext string, hash []byte) bool {
	got := HashAPIKey(plaintext)
	if len(hash) != len(got) {
		subtle.ConstantTimeCompare(got, make([]byte, len(got)))
		return false
	}

	return subtle.ConstantTimeCompare(got, hash) == 1
}

func validAPIKeyFormat(plaintext string) bool {
	const totalLen = len(APIKeyTokenPrefix) + apiKeyPrefixLen + 1 + apiKeySecretLen
	if len(plaintext) != totalLen {
		return false
	}

	if !strings.HasPrefix(plaintext, APIKeyTokenPrefix) {
		return false
	}

	underscore := len(APIKeyTokenPrefix) + apiKeyPrefixLen
	if plaintext[underscore] != '_' {
		return false
	}

	prefix := plaintext[len(APIKeyTokenPrefix):underscore]
	secret := plaintext[underscore+1:]

	return isHex(prefix) && isHex(secret)
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}

		if c >= 'a' && c <= 'f' {
			continue
		}

		return false
	}

	return true
}
