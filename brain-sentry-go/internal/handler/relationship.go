package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/service"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// RelationshipHandler handles memory relationship endpoints.
type RelationshipHandler struct {
	relService    *service.RelationshipService
	memoryService *service.MemoryService
}

// NewRelationshipHandler creates a new RelationshipHandler.
func NewRelationshipHandler(relService *service.RelationshipService, memoryService *service.MemoryService) *RelationshipHandler {
	return &RelationshipHandler{
		relService:    relService,
		memoryService: memoryService,
	}
}

// List handles GET /v1/relationships
func (h *RelationshipHandler) List(w http.ResponseWriter, r *http.Request) {
	rels, err := h.relService.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list relationships")
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

// Create handles POST /v1/relationships
func (h *RelationshipHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromMemoryID string `json:"fromMemoryId"`
		ToMemoryID   string `json:"toMemoryId"`
		Type         string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FromMemoryID == "" || req.ToMemoryID == "" {
		writeError(w, http.StatusBadRequest, "fromMemoryId and toMemoryId are required")
		return
	}

	relType := parseRelType(req.Type)

	rel, err := h.relService.CreateRelationship(r.Context(), req.FromMemoryID, req.ToMemoryID, relType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create relationship")
		return
	}

	writeJSON(w, http.StatusCreated, rel)
}

// CreateBidirectional handles POST /v1/relationships/bidirectional
func (h *RelationshipHandler) CreateBidirectional(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryID1 string `json:"memoryId1"`
		MemoryID2 string `json:"memoryId2"`
		Type1     string `json:"type1"`
		Type2     string `json:"type2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.relService.CreateBidirectional(r.Context(), req.MemoryID1, req.MemoryID2, parseRelType(req.Type1), parseRelType(req.Type2))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create bidirectional relationship")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "bidirectional relationship created"})
}

// GetFrom handles GET /v1/relationships/from/{memoryId}
func (h *RelationshipHandler) GetFrom(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	rels, err := h.relService.GetRelationshipsFrom(r.Context(), memoryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get relationships")
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

// GetTo handles GET /v1/relationships/to/{memoryId}
func (h *RelationshipHandler) GetTo(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	rels, err := h.relService.GetRelationshipsTo(r.Context(), memoryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get relationships")
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

// GetBetween handles GET /v1/relationships/between
func (h *RelationshipHandler) GetBetween(w http.ResponseWriter, r *http.Request) {
	fromID := r.URL.Query().Get("from")
	toID := r.URL.Query().Get("to")
	if fromID == "" || toID == "" {
		writeError(w, http.StatusBadRequest, "from and to query params are required")
		return
	}

	rel, err := h.relService.GetRelationship(r.Context(), fromID, toID)
	if err != nil {
		writeError(w, http.StatusNotFound, "relationship not found")
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

// GetRelated handles GET /v1/relationships/{memoryId}/related
func (h *RelationshipHandler) GetRelated(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	minStrength := 0.0
	if ms := r.URL.Query().Get("minStrength"); ms != "" {
		minStrength, _ = strconv.ParseFloat(ms, 64)
	}

	rels, err := h.relService.FindRelatedMemories(r.Context(), memoryID, minStrength)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find related memories")
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

// UpdateStrength handles PUT /v1/relationships/{relationshipId}/strength
func (h *RelationshipHandler) UpdateStrength(w http.ResponseWriter, r *http.Request) {
	relID := chi.URLParam(r, "relationshipId")

	var req struct {
		Strength float64 `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rel, err := h.relService.UpdateStrength(r.Context(), relID, req.Strength)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update strength")
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

// DeleteBetween handles DELETE /v1/relationships/between
func (h *RelationshipHandler) DeleteBetween(w http.ResponseWriter, r *http.Request) {
	fromID := r.URL.Query().Get("from")
	toID := r.URL.Query().Get("to")
	if fromID == "" || toID == "" {
		writeError(w, http.StatusBadRequest, "from and to query params are required")
		return
	}

	if err := h.relService.DeleteRelationship(r.Context(), fromID, toID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete relationship")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "relationship deleted"})
}

// DeleteAll handles DELETE /v1/relationships/{memoryId}
func (h *RelationshipHandler) DeleteAll(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	if err := h.relService.DeleteAllForMemory(r.Context(), memoryID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete relationships")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "all relationships deleted"})
}

// Suggest handles POST /v1/relationships/{memoryId}/suggest
func (h *RelationshipHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")

	m, err := h.memoryService.GetMemory(r.Context(), memoryID)
	if err != nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}

	// r.Context() is cancelled the moment this handler returns
	// (writeJSON below is synchronous, so the request completes before the
	// goroutine's first call). Building a background context that preserves
	// the tenant lets DetectAndCreateRelationships actually run — without
	// this fix every call here was a silent no-op: the first DB query in
	// the goroutine failed with "full text search: context canceled"
	// before any relationship was detected or created. Surfaced by the
	// sales-corpus-llm validation scenario.
	bgCtx := tenant.WithTenant(context.Background(), tenant.FromContext(r.Context()))
	go h.relService.DetectAndCreateRelationships(bgCtx, m)

	writeJSON(w, http.StatusAccepted, map[string]string{"message": "relationship detection started"})
}

func parseRelType(t string) domain.RelationshipType {
	switch domain.RelationshipType(t) {
	case domain.RelUsedWith, domain.RelConflictsWith, domain.RelSupersedes,
		domain.RelRelatedTo, domain.RelRequires, domain.RelPartOf:
		return domain.RelationshipType(t)
	default:
		return domain.RelRelatedTo
	}
}
