package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// TenantHandler handles tenant management endpoints.
type TenantHandler struct {
	tenantRepo *postgres.TenantRepository
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(tenantRepo *postgres.TenantRepository) *TenantHandler {
	return &TenantHandler{tenantRepo: tenantRepo}
}

// List handles GET /v1/tenants
func (h *TenantHandler) List(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.tenantRepo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}
	writeJSON(w, http.StatusOK, tenants)
}

// GetByID handles GET /v1/tenants/{id}
func (h *TenantHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.tenantRepo.FindByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// Create handles POST /v1/tenants
func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	t := &domain.Tenant{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Active:      true,
		MaxMemories: req.MaxMemories,
		MaxUsers:    req.MaxUsers,
	}
	if req.Settings != nil {
		settingsJSON, _ := json.Marshal(req.Settings)
		t.Settings = settingsJSON
	}

	if err := h.tenantRepo.Create(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tenant")
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

// Update handles PUT /v1/tenants/{id}
func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.tenantRepo.FindByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	var req dto.CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Slug != "" {
		existing.Slug = req.Slug
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	existing.MaxMemories = req.MaxMemories
	existing.MaxUsers = req.MaxUsers
	if req.Settings != nil {
		settingsJSON, _ := json.Marshal(req.Settings)
		existing.Settings = settingsJSON
	}

	if err := h.tenantRepo.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update tenant")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /v1/tenants/{id}
func (h *TenantHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.tenantRepo.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete tenant")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "tenant deleted"})
}

// Stats handles GET /v1/tenants/stats
//
//	@Summary		Per-tenant counts for the admin UI
//	@Description	Memórias, usuários e relacionamentos por tenant. A tela de tenants já consumia esta rota, que não existia e devolvia 404.
//	@Tags			Tenants
//	@Produce		json
//	@Success		200	{array}		postgres.TenantStats
//	@Security		BearerAuth
//	@Router			/v1/tenants/stats [get]
func (h *TenantHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.tenantRepo.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute tenant stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
