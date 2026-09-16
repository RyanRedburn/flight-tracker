package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxAPIKeyCreateAttempts = 5

type KeysHandler struct {
	store store.Store
}

func NewKeysHandler(s store.Store) *KeysHandler {
	return &KeysHandler{store: s}
}

// APIKeyResponse is API key metadata without the plaintext secret or hash.
type APIKeyResponse struct {
	ID        string           `json:"id"`
	Prefix    string           `json:"prefix"`
	Role      model.APIKeyRole `json:"role"`
	Name      string           `json:"name,omitempty"`
	CreatedAt string           `json:"created_at"`
	RevokedAt *string          `json:"revoked_at,omitempty"`
}

// CreatedAPIKeyResponse includes the plaintext key, which is returned only once.
type CreatedAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}

// Create issues a new API key. The plaintext secret is returned only in this response.
//
//	@Summary		Create API key
//	@Description	Creates a hashed API key. The plaintext key is returned once; subsequent authentication uses Authorization: Bearer or X-API-Key.
//	@Tags			keys,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.CreateAPIKeyRequest	true	"Role and optional name"
//	@Success		201		{object}	CreatedAPIKeyResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		429		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/keys [post]
func (h *KeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	now := time.Now().UTC()
	name := strings.TrimSpace(req.Name)

	var (
		created   *model.APIKey
		plaintext string
	)

	for range maxAPIKeyCreateAttempts {
		secret, prefix, err := model.GenerateAPIKey()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateAPIKey})
			return
		}

		key := &model.APIKey{
			ID:        uuid.NewString(),
			Prefix:    prefix,
			KeyHash:   model.HashAPIKey(secret),
			Role:      model.APIKeyRole(req.Role),
			Name:      name,
			CreatedAt: now,
		}

		err = h.store.CreateAPIKey(r.Context(), key)
		if err == nil {
			created = key
			plaintext = secret

			break
		}

		if !errors.Is(err, store.ErrConflict) {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateAPIKey})
			return
		}
	}

	if created == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateAPIKey})
		return
	}

	resp := CreatedAPIKeyResponse{
		APIKeyResponse: toAPIKeyResponse(created),
		Key:            plaintext,
	}
	writeJSON(w, http.StatusCreated, resp)
}

// List returns API key metadata (never hashes or plaintext secrets).
//
//	@Summary		List API keys
//	@Description	Returns all API keys, including revoked keys. Hashes and plaintext secrets are never included.
//	@Tags			keys,internal
//	@Produce		json
//	@Success		200	{array}		APIKeyResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		429	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/keys [get]
func (h *KeysHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListAPIKeys(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedListAPIKeys})
		return
	}

	out := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		out = append(out, toAPIKeyResponse(key))
	}

	writeJSON(w, http.StatusOK, out)
}

// Revoke marks an API key unusable. The operation is idempotent if already revoked.
//
//	@Summary		Revoke API key
//	@Description	Revokes an API key. Already-revoked keys return the current metadata.
//	@Tags			keys,internal
//	@Produce		json
//	@Param			id	path		string	true	"API key ID"
//	@Success		200	{object}	APIKeyResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		429	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/keys/{id}/revoke [post]
func (h *KeysHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errAPIKeyIDRequired})
		return
	}

	if err := h.store.RevokeAPIKey(r.Context(), id, time.Now().UTC()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: errAPIKeyNotFound})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedRevokeAPIKey})

		return
	}

	key, err := h.store.GetAPIKey(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: errAPIKeyNotFound})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedRevokeAPIKey})

		return
	}

	writeJSON(w, http.StatusOK, toAPIKeyResponse(key))
}

func toAPIKeyResponse(key *model.APIKey) APIKeyResponse {
	resp := APIKeyResponse{
		ID:        key.ID,
		Prefix:    key.Prefix,
		Role:      key.Role,
		Name:      key.Name,
		CreatedAt: key.CreatedAt.UTC().Format(time.RFC3339),
	}

	if key.RevokedAt != nil {
		revoked := key.RevokedAt.UTC().Format(time.RFC3339)
		resp.RevokedAt = &revoked
	}

	return resp
}
