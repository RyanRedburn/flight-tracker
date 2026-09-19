package model

import (
	"bytes"
	"testing"
	"time"
)

func TestGenerateParseAndVerifyAPIKey(t *testing.T) {
	plaintext, prefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	gotPrefix, err := ParseAPIKeyPrefix(plaintext)
	if err != nil {
		t.Fatalf("ParseAPIKeyPrefix: %v", err)
	}

	if gotPrefix != prefix {
		t.Fatalf("prefix = %q, want %q", gotPrefix, prefix)
	}

	hash := HashAPIKey(plaintext)
	if !VerifyAPIKey(plaintext, hash) {
		t.Fatal("VerifyAPIKey() = false, want true")
	}

	other, _, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey other: %v", err)
	}

	if VerifyAPIKey(other, hash) {
		t.Fatal("VerifyAPIKey(other) = true, want false")
	}

	if VerifyAPIKey(plaintext, hash[:len(hash)-1]) {
		t.Fatal("VerifyAPIKey(short hash) = true, want false")
	}

	if bytes.Equal(hash, HashAPIKey(other)) {
		t.Fatal("distinct keys produced the same hash")
	}
}

func TestParseAPIKeyPrefixInvalid(t *testing.T) {
	tests := []string{
		"",
		"ftk_short",
		"ftk_aabbccdd_not-hex-secret-value-000000000000",
		"FTK_aabbccdd_0123456789abcdef0123456789abcdef",
		"ftk_AABBCCDD_0123456789abcdef0123456789abcdef",
		"ftk_aabbccdd0123456789abcdef0123456789abcdef00",
	}

	for _, raw := range tests {
		if _, err := ParseAPIKeyPrefix(raw); err == nil {
			t.Errorf("ParseAPIKeyPrefix(%q) expected error", raw)
		}
	}
}

func TestAPIKeyRoleValid(t *testing.T) {
	tests := []struct {
		role APIKeyRole
		want bool
	}{
		{APIKeyRoleConsumer, true},
		{APIKeyRoleSubscriber, true},
		{APIKeyRoleAdmin, true},
		{APIKeyRole("operator"), false},
		{APIKeyRole(""), false},
	}

	for _, tt := range tests {
		if got := tt.role.Valid(); got != tt.want {
			t.Errorf("APIKeyRole(%q).Valid() = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestRolePermits(t *testing.T) {
	external := []APIKeyRole{APIKeyRoleConsumer, APIKeyRoleSubscriber, APIKeyRoleAdmin}
	admin := []APIKeyRole{APIKeyRoleAdmin}

	tests := []struct {
		have    APIKeyRole
		allowed []APIKeyRole
		want    bool
	}{
		{APIKeyRoleConsumer, external, true},
		{APIKeyRoleSubscriber, external, true},
		{APIKeyRoleAdmin, external, true},
		{APIKeyRoleConsumer, admin, false},
		{APIKeyRoleSubscriber, admin, false},
		{APIKeyRoleAdmin, admin, true},
	}

	for _, tt := range tests {
		if got := RolePermits(tt.have, tt.allowed); got != tt.want {
			t.Errorf("RolePermits(%q) = %v, want %v", tt.have, got, tt.want)
		}
	}
}

func TestCreateAPIKeyRequestValidate(t *testing.T) {
	if err := (CreateAPIKeyRequest{Role: "consumer"}).Validate(); err != nil {
		t.Fatalf("valid consumer: %v", err)
	}

	if err := (CreateAPIKeyRequest{Role: "nope"}).Validate(); err == nil {
		t.Fatal("expected invalid role error")
	}

	longName := make([]byte, 101)
	for i := range longName {
		longName[i] = 'a'
	}

	if err := (CreateAPIKeyRequest{Role: "admin", Name: string(longName)}).Validate(); err == nil {
		t.Fatal("expected long name error")
	}
}

func TestAPIKeyRevoked(t *testing.T) {
	key := APIKey{}
	if key.Revoked() {
		t.Fatal("zero key should not be revoked")
	}

	now := time.Now().UTC()

	key.RevokedAt = &now
	if !key.Revoked() {
		t.Fatal("key with RevokedAt should be revoked")
	}
}
