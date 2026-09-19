package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/uuid"
)

const bootstrapKeyName = "bootstrap"

var errAuthEnabledNoKeys = errors.New("auth is enabled but api_keys is empty; set AUTH_BOOTSTRAP_ADMIN_KEY or create a key with AUTH_DISABLED=true first")

// BootstrapAdminKey inserts the configured plaintext key as an admin credential
// when the api_keys table is empty. Concurrent replicas that race on the first
// insert treat a unique conflict as success. The plaintext is never logged.
func BootstrapAdminKey(ctx context.Context, st store.Store, plaintext string) (created bool, prefix string, err error) {
	if plaintext == "" {
		return false, "", nil
	}

	prefix, err = model.ParseAPIKeyPrefix(plaintext)
	if err != nil {
		return false, "", fmt.Errorf("AUTH_BOOTSTRAP_ADMIN_KEY: %w", err)
	}

	n, err := st.CountAPIKeys(ctx)
	if err != nil {
		return false, "", err
	}

	if n > 0 {
		return false, prefix, nil
	}

	now := time.Now().UTC()
	key := &model.APIKey{
		ID:        uuid.NewString(),
		Prefix:    prefix,
		KeyHash:   model.HashAPIKey(plaintext),
		Role:      model.APIKeyRoleAdmin,
		Name:      bootstrapKeyName,
		CreatedAt: now,
	}

	if err := st.CreateAPIKey(ctx, key); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return false, prefix, nil
		}

		return false, "", err
	}

	return true, prefix, nil
}

func RequireAPIKeysIfAuthEnabled(ctx context.Context, st store.Store, authDisabled bool) error {
	if authDisabled {
		return nil
	}

	n, err := st.CountAPIKeys(ctx)
	if err != nil {
		return err
	}

	if n == 0 {
		return errAuthEnabledNoKeys
	}

	return nil
}
