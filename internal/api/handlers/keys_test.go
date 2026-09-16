package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"

	"github.com/go-chi/chi/v5"
)

const testAPIKeyID = "key-1"

func TestKeysCreateReturnsPlaintextOnce(t *testing.T) {
	var stored *model.APIKey

	h := NewKeysHandler(&storetest.Stub{
		CreateAPIKeyFn: func(_ context.Context, key *model.APIKey) error {
			copied := *key
			copied.KeyHash = append([]byte(nil), key.KeyHash...)
			stored = &copied

			return nil
		},
	})

	body, err := json.Marshal(model.CreateAPIKeyRequest{Role: "consumer", Name: "docs"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}

	var resp CreatedAPIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Key == "" {
		t.Fatal("plaintext key missing")
	}

	if resp.Role != model.APIKeyRoleConsumer {
		t.Errorf("role = %q", resp.Role)
	}

	if stored == nil {
		t.Fatal("expected stored key")
	}

	if !model.VerifyAPIKey(resp.Key, stored.KeyHash) {
		t.Fatal("stored hash does not match returned plaintext")
	}

	if stored.Prefix != resp.Prefix {
		t.Errorf("prefix = %q, stored %q", resp.Prefix, stored.Prefix)
	}
}

func TestKeysCreateInvalidRole(t *testing.T) {
	h := NewKeysHandler(&storetest.Stub{})

	body, err := json.Marshal(model.CreateAPIKeyRequest{Role: "root"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestKeysListOmitsHash(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := NewKeysHandler(&storetest.Stub{
		ListAPIKeysFn: func(context.Context) ([]*model.APIKey, error) {
			return []*model.APIKey{{
				ID:        testAPIKeyID,
				Prefix:    "aabbccdd",
				KeyHash:   []byte("secret-hash"),
				Role:      model.APIKeyRoleAdmin,
				CreatedAt: now,
			}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	raw := rec.Body.String()
	if bytes.Contains([]byte(raw), []byte("secret-hash")) {
		t.Fatal("list leaked key hash")
	}

	var keys []APIKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(keys) != 1 || keys[0].ID != testAPIKeyID {
		t.Fatalf("keys = %+v", keys)
	}
}

func TestKeysRevoke(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	revokedAt := now.Add(time.Hour)

	var revokedID string

	h := NewKeysHandler(&storetest.Stub{
		RevokeAPIKeyFn: func(_ context.Context, id string, at time.Time) error {
			revokedID = id
			revokedAt = at

			return nil
		},
		GetAPIKeyFn: func(_ context.Context, id string) (*model.APIKey, error) {
			return &model.APIKey{
				ID:        id,
				Prefix:    "aabbccdd",
				Role:      model.APIKeyRoleAdmin,
				CreatedAt: now,
				RevokedAt: &revokedAt,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/"+testAPIKeyID+"/revoke", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", testAPIKeyID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Revoke(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	if revokedID != testAPIKeyID {
		t.Fatalf("revoked id = %q", revokedID)
	}

	var resp APIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.RevokedAt == nil {
		t.Fatal("expected revoked_at")
	}
}

func TestKeysRevokeNotFound(t *testing.T) {
	h := NewKeysHandler(&storetest.Stub{
		RevokeAPIKeyFn: func(context.Context, string, time.Time) error {
			return store.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/missing/revoke", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Revoke(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
