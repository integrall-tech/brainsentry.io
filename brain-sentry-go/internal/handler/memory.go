package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/eval"
	"github.com/integraltech/brainsentry/internal/service"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// MemoryHandler handles memory endpoints.
type MemoryHandler struct {
	memoryService       *service.MemoryService
	relationshipService *service.RelationshipService
	evalStore           *eval.Store
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(memoryService *service.MemoryService, relationshipService *service.RelationshipService) *MemoryHandler {
	return &MemoryHandler{memoryService: memoryService, relationshipService: relationshipService}
}

// WithEvalCapture attaches the eval candidate store. When the env flag
// BRAINSENTRY_EVAL_CAPTURE=1 is set, every search call will record the
// query + top-N results into the store for later export/replay.
func (h *MemoryHandler) WithEvalCapture(store *eval.Store) *MemoryHandler {
	h.evalStore = store
	return h
}

// Create handles POST /v1/memories
//
//	@Summary		Create a new memory
//	@Tags			Memories
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CreateMemoryRequest	true	"Memory data"
//	@Success		201		{object}	domain.Memory
//	@Failure		400		{object}	dto.ErrorResponse
//	@Security		BearerAuth
//	@Router			/v1/memories [post]
func (h *MemoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	m, err := h.memoryService.CreateMemory(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create memory")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

// GetByID handles GET /v1/memories/{id}
//
//	@Summary		Get memory by ID
//	@Tags			Memories
//	@Produce		json
//	@Param			id	path		string	true	"Memory ID"
//	@Success		200	{object}	domain.Memory
//	@Failure		404	{object}	dto.ErrorResponse
//	@Security		BearerAuth
//	@Router			/v1/memories/{id} [get]
func (h *MemoryHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.memoryService.GetMemory(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var metadata map[string]any
	if m.Metadata != nil {
		_ = json.Unmarshal(m.Metadata, &metadata)
	}

	resp := dto.MemoryResponse{
		ID:                  m.ID,
		Content:             m.Content,
		Summary:             m.Summary,
		Category:            m.Category,
		Importance:          m.Importance,
		ValidationStatus:    m.ValidationStatus,
		Metadata:            metadata,
		Tags:                m.Tags,
		SourceType:          m.SourceType,
		SourceReference:     m.SourceReference,
		CreatedBy:           m.CreatedBy,
		TenantID:            m.TenantID,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		LastAccessedAt:      m.LastAccessedAt,
		Version:             m.Version,
		AccessCount:         m.AccessCount,
		InjectionCount:      m.InjectionCount,
		HelpfulCount:        m.HelpfulCount,
		NotHelpfulCount:     m.NotHelpfulCount,
		HelpfulnessRate:     m.HelpfulnessRate(),
		RelevanceScore:      m.RelevanceScore(),
		CodeExample:         m.CodeExample,
		ProgrammingLanguage: m.ProgrammingLanguage,
		MemoryType:          m.MemoryType,
		EmotionalWeight:     m.EmotionalWeight,
		SimHash:             m.SimHash,
		ValidFrom:           m.ValidFrom,
		ValidTo:             m.ValidTo,
		DecayRate:           m.DecayRate,
		SupersededBy:        m.SupersededBy,
		DecayedRelevance:    service.ComputeDecayedRelevance(m, time.Now()),
	}

	// Populate related memories if relationship service is available
	if h.relationshipService != nil {
		rels, err := h.relationshipService.FindRelatedMemories(r.Context(), id, 0.0)
		if err == nil && len(rels) > 0 {
			refs := make([]dto.RelatedMemoryRef, 0, len(rels))
			for _, rel := range rels {
				targetID := rel.ToMemoryID
				if targetID == id {
					targetID = rel.FromMemoryID
				}
				ref := dto.RelatedMemoryRef{
					ID:               targetID,
					RelationshipType: rel.Type,
					Strength:         rel.Strength,
				}
				// Try to get summary
				related, err := h.memoryService.GetMemory(r.Context(), targetID)
				if err == nil {
					ref.Summary = related.Summary
				}
				refs = append(refs, ref)
			}
			resp.RelatedMemories = refs
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// List handles GET /v1/memories
//
//	@Summary		List memories
//	@Tags			Memories
//	@Produce		json
//	@Param			page	query		int	false	"Page number"	default(0)
//	@Param			size	query		int	false	"Page size"		default(20)
//	@Success		200		{object}	dto.MemoryListResponse
//	@Security		BearerAuth
//	@Router			/v1/memories [get]
func (h *MemoryHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = 20
	}

	resp, err := h.memoryService.ListMemories(r.Context(), page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list memories")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Update handles PUT /v1/memories/{id}
func (h *MemoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req dto.UpdateMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	m, err := h.memoryService.UpdateMemory(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update memory")
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// Delete handles DELETE /v1/memories/{id}
func (h *MemoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.memoryService.DeleteMemory(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete memory")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "memory deleted"})
}

// Search handles POST /v1/memories/search
//
//	@Summary		Search memories
//	@Tags			Memories
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.SearchRequest	true	"Search query"
//	@Success		200		{array}		dto.MemoryResponse
//	@Security		BearerAuth
//	@Router			/v1/memories/search [post]
func (h *MemoryHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req dto.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	start := time.Now()
	searchResp, err := h.memoryService.SearchMemories(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	if h.evalStore != nil && eval.CaptureEnabled() {
		ids := make([]string, 0, len(searchResp.Results))
		for _, r := range searchResp.Results {
			ids = append(ids, r.ID)
		}
		k := req.Limit
		if k <= 0 {
			k = len(ids)
		}
		h.evalStore.Add(eval.Candidate{
			TenantID:  tenant.FromContext(r.Context()),
			Query:     req.Query,
			K:         k,
			TopIDs:    ids,
			LatencyMs: time.Since(start).Milliseconds(),
			Strategy:  "search",
		})
	}

	writeJSON(w, http.StatusOK, searchResp)
}

// GetByCategory handles GET /v1/memories/by-category/{category}
func (h *MemoryHandler) GetByCategory(w http.ResponseWriter, r *http.Request) {
	category := domain.MemoryCategory(chi.URLParam(r, "category"))
	results, err := h.memoryService.GetByCategory(r.Context(), category)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get by category")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// GetByImportance handles GET /v1/memories/by-importance/{importance}
func (h *MemoryHandler) GetByImportance(w http.ResponseWriter, r *http.Request) {
	importance := domain.ImportanceLevel(chi.URLParam(r, "importance"))
	results, err := h.memoryService.GetByImportance(r.Context(), importance)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get by importance")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// Versions handles GET /v1/memories/{id}/versions
func (h *MemoryHandler) Versions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	versions, err := h.memoryService.GetVersionHistory(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get version history")
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// Feedback handles POST /v1/memories/{id}/feedback
func (h *MemoryHandler) Feedback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Helpful bool `json:"helpful"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.memoryService.RecordFeedback(r.Context(), id, req.Helpful); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record feedback")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "feedback recorded"})
}
