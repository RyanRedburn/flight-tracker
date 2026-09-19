package api

import (
	"context"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestBootstrapAdminKeyEmpty(t *testing.T) {
	created, prefix, err := BootstrapAdminKey(context.Background(), &storetest.Stub{}, "")
	if err != nil {
		t.Fatalf("BootstrapAdminKey: %v", err)
	}

	if created || prefix != "" {
		t.Fatalf("created=%v prefix=%q, want skipped", created, prefix)
	}
}

func TestBootstrapAdminKeyInsertsWhenEmpty(t *testing.T) {
	plaintext, wantPrefix, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	var stored *model.APIKey

	stub := &storetest.Stub{
		CountAPIKeysFn: func(context.Context) (int64, error) { return 0, nil },
		CreateAPIKeyFn: func(_ context.Context, key *model.APIKey) error {
			copied := *key
			stored = &copied

			return nil
		},
	}

	created, prefix, err := BootstrapAdminKey(context.Background(), stub, plaintext)
	if err != nil {
		t.Fatalf("BootstrapAdminKey: %v", err)
	}

	if !created {
		t.Fatal("expected created")
	}

	if prefix != wantPrefix {
		t.Fatalf("prefix = %q, want %q", prefix, wantPrefix)
	}

	if stored == nil || stored.Role != model.APIKeyRoleAdmin {
		t.Fatalf("stored = %+v", stored)
	}

	if !model.VerifyAPIKey(plaintext, stored.KeyHash) {
		t.Fatal("bootstrap hash mismatch")
	}
}

func TestBootstrapAdminKeySkipsWhenKeysExist(t *testing.T) {
	plaintext, _, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	stub := &storetest.Stub{
		CountAPIKeysFn: func(context.Context) (int64, error) { return 1, nil },
	}

	created, _, err := BootstrapAdminKey(context.Background(), stub, plaintext)
	if err != nil {
		t.Fatalf("BootstrapAdminKey: %v", err)
	}

	if created {
		t.Fatal("expected skip when keys exist")
	}
}

func TestBootstrapAdminKeyConflict(t *testing.T) {
	plaintext, _, err := model.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	stub := &storetest.Stub{
		CountAPIKeysFn: func(context.Context) (int64, error) { return 0, nil },
		CreateAPIKeyFn: func(context.Context, *model.APIKey) error {
			return store.ErrConflict
		},
	}

	created, _, err := BootstrapAdminKey(context.Background(), stub, plaintext)
	if err != nil {
		t.Fatalf("BootstrapAdminKey: %v", err)
	}

	if created {
		t.Fatal("conflict should be treated as another replica winning")
	}
}

func TestRequireAPIKeysIfAuthEnabled(t *testing.T) {
	if err := RequireAPIKeysIfAuthEnabled(context.Background(), &storetest.Stub{}, true); err != nil {
		t.Fatalf("disabled: %v", err)
	}

	empty := &storetest.Stub{
		CountAPIKeysFn: func(context.Context) (int64, error) { return 0, nil },
	}
	if err := RequireAPIKeysIfAuthEnabled(context.Background(), empty, false); !errors.Is(err, errAuthEnabledNoKeys) {
		t.Fatalf("empty table error = %v", err)
	}

	present := &storetest.Stub{
		CountAPIKeysFn: func(context.Context) (int64, error) { return 2, nil },
	}
	if err := RequireAPIKeysIfAuthEnabled(context.Background(), present, false); err != nil {
		t.Fatalf("present: %v", err)
	}
}
