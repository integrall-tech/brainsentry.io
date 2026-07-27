package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/middleware"
	"github.com/integraltech/brainsentry/internal/service"
)

// APIKeyHandler exposes service-key management. Every route here is
// admin-only AND closed to service keys themselves (see
// middleware.RejectServicePrincipal at the router).
type APIKeyHandler struct {
	apiKeys *service.APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(apiKeys *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeys: apiKeys}
}

type createAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// createAPIKeyResponse is the ONLY place the plaintext secret appears. It is
// not stored and cannot be recovered — a lost key is replaced, not read.
type createAPIKeyResponse struct {
	*domain.APIKey
	Secret  string `json:"secret"`
	Warning string `json:"warning"`
}

// Create handles POST /v1/tenants/{id}/api-keys
//
//	@Summary		Issue a service API key for a tenant
//	@Description	Returns the secret ONCE — it is stored only as a hash and cannot be recovered. Admin JWT only; service keys are refused here.
//	@Tags			API Keys
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Tenant ID"
//	@Param			request	body		createAPIKeyRequest		true	"Key name and optional expiry"
//	@Success		201		{object}	createAPIKeyResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		403		{object}	dto.ErrorResponse
//	@Security		BearerAuth
//	@Router			/v1/tenants/{id}/api-keys [post]
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant id is required")
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var createdBy string
	if claims := middleware.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = claims.UserID
	}

	created, err := h.apiKeys.Create(r.Context(), tenantID, req.Name, createdBy, req.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, createAPIKeyResponse{
		APIKey:  created.Key,
		Secret:  created.Secret,
		Warning: "store this secret now — it is not recoverable",
	})
}

// List handles GET /v1/tenants/{id}/api-keys
//
//	@Summary		List a tenant's service API keys
//	@Description	Includes revoked keys, so an operator can see what was revoked and when. Never returns secrets.
//	@Tags			API Keys
//	@Produce		json
//	@Param			id	path		string	true	"Tenant ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		403	{object}	dto.ErrorResponse
//	@Security		BearerAuth
//	@Router			/v1/tenants/{id}/api-keys [get]
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant id is required")
		return
	}

	keys, err := h.apiKeys.List(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}
	if keys == nil {
		keys = []*domain.APIKey{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"apiKeys": keys,
		"total":   len(keys),
	})
}

// Revoke handles DELETE /v1/api-keys/{keyId}
//
//	@Summary		Revoke a service API key
//	@Description	Immediate and idempotent. The row is kept as a tombstone so the audit trail of which key acted survives.
//	@Tags			API Keys
//	@Produce		json
//	@Param			keyId	path		string	true	"API key ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	dto.ErrorResponse
//	@Security		BearerAuth
//	@Router			/v1/api-keys/{keyId} [delete]
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyId")
	if keyID == "" {
		writeError(w, http.StatusBadRequest, "key id is required")
		return
	}

	if err := h.apiKeys.Revoke(r.Context(), keyID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"revoked": true, "id": keyID})
}
