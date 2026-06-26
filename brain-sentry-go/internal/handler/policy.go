package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/service"
)

// PolicyHandler exposes CRUD + enforcement endpoints for Policies.
type PolicyHandler struct {
	engine   *service.PolicyEngine
	decision *service.DecisionService
}

// NewPolicyHandler builds the handler.
func NewPolicyHandler(engine *service.PolicyEngine, decision *service.DecisionService) *PolicyHandler {
	return &PolicyHandler{engine: engine, decision: decision}
}

// List handles GET /v1/policies
func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "policy engine not available")
		return
	}
	list, err := h.engine.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":    len(list),
		"policies": list,
	})
}

// Get handles GET /v1/policies/{id}
func (h *PolicyHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "policy engine not available")
		return
	}
	id := chi.URLParam(r, "id")
	p, err := h.engine.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

// Create handles POST /v1/policies
func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "policy engine not available")
		return
	}
	var req service.CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.engine.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

// Update handles PUT /v1/policies/{id}
func (h *PolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "policy engine not available")
		return
	}
	id := chi.URLParam(r, "id")
	var req service.CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.engine.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

// Delete handles DELETE /v1/policies/{id}
func (h *PolicyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "policy engine not available")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.engine.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EnforceOnDecision handles POST /v1/policies/enforce
// Body: { decisionId }
func (h *PolicyHandler) EnforceOnDecision(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "policy engine not available")
		return
	}
	var req struct {
		DecisionID string `json:"decisionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DecisionID == "" {
		writeError(w, http.StatusBadRequest, "decisionId is required")
		return
	}
	if h.decision == nil {
		writeError(w, http.StatusServiceUnavailable, "decision service not available")
		return
	}
	d, err := h.decision.Get(r.Context(), req.DecisionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	violations := h.engine.ExplainDecision(r.Context(), d)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"decision":   d,
		"violations": violations,
		"compliant":  len(violations) == 0,
	})
}
