package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/service"
)

// ConflictHandler handles conflict detection endpoints.
type ConflictHandler struct {
	conflictService *service.ConflictService
}

// NewConflictHandler creates a new ConflictHandler.
func NewConflictHandler(conflictService *service.ConflictService) *ConflictHandler {
	return &ConflictHandler{conflictService: conflictService}
}

// DetectForMemory handles POST /v1/conflicts/detect/{memoryId}
func (h *ConflictHandler) DetectForMemory(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	if memoryID == "" {
		writeError(w, http.StatusBadRequest, "memoryId is required")
		return
	}

	conflicts, err := h.conflictService.DetectConflicts(r.Context(), memoryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, conflicts)
}

// ScanAll handles POST /v1/conflicts/scan
func (h *ConflictHandler) ScanAll(w http.ResponseWriter, r *http.Request) {
	conflicts, err := h.conflictService.ScanAllConflicts(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, conflicts)
}

// NearDuplicates handles GET /v1/conflicts/near-duplicates
// Query params: strategy=jaro_winkler|blocking_jw|semantic, threshold=0..1, limit=N
func (h *ConflictHandler) NearDuplicates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	strategy := service.DedupStrategy(q.Get("strategy"))
	if strategy == "" {
		strategy = service.DedupBlockingJW
	}
	threshold := 0.85
	if v := q.Get("threshold"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}
	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	pairs, err := h.conflictService.FindNearDuplicates(r.Context(), strategy, threshold, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"strategy":  strategy,
		"threshold": threshold,
		"count":     len(pairs),
		"pairs":     pairs,
	})
}

// Resolve handles POST /v1/conflicts/resolve — applies a human verdict to
// a conflicting pair. Body:
//
//	{ "winnerId": "...", "loserId": "...", "action": "supersede"|"dismiss" }
//
// supersede marks the loser superseded by the winner (and promotes the
// winner to CORRECTED provenance); dismiss keeps both. This is the
// interactive counterpart to the (automatic) detection endpoints.
func (h *ConflictHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WinnerID string `json:"winnerId"`
		LoserID  string `json:"loserId"`
		Action   string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	action, err := service.ParseResolutionAction(req.Action)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.conflictService.ResolveConflict(r.Context(), req.WinnerID, req.LoserID, action); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"winnerId": req.WinnerID,
		"loserId":  req.LoserID,
		"action":   action,
		"resolved": true,
	})
}
