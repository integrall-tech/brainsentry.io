package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/service"
)

// AuditHandler handles audit log endpoints.
type AuditHandler struct {
	auditService *service.AuditService
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

func getLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	return limit
}

// List handles GET /v1/audit-logs
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	logs, err := h.auditService.ListByTenant(r.Context(), getLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// ByEventType handles GET /v1/audit-logs/by-event/{eventType}
func (h *AuditHandler) ByEventType(w http.ResponseWriter, r *http.Request) {
	eventType := chi.URLParam(r, "eventType")
	logs, err := h.auditService.FindByEventType(r.Context(), eventType, getLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// ByUser handles GET /v1/audit-logs/by-user/{userId}
func (h *AuditHandler) ByUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	logs, err := h.auditService.FindByUserID(r.Context(), userID, getLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// BySession handles GET /v1/audit-logs/by-session/{sessionId}
func (h *AuditHandler) BySession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	logs, err := h.auditService.FindBySessionID(r.Context(), sessionID, getLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// Recent handles GET /v1/audit-logs/recent
func (h *AuditHandler) Recent(w http.ResponseWriter, r *http.Request) {
	logs, err := h.auditService.FindRecent(r.Context(), getLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find recent audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// ByDateRange handles GET /v1/audit-logs/by-date-range
func (h *AuditHandler) ByDateRange(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid 'from' date format, use RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid 'to' date format, use RFC3339")
		return
	}

	logs, err2 := h.auditService.FindByDateRange(r.Context(), from, to, getLimit(r))
	if err2 != nil {
		writeError(w, http.StatusInternalServerError, "failed to find audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// ByMemory handles GET /v1/audit/memory/{memoryId}/history
func (h *AuditHandler) ByMemory(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "memoryId")
	if memoryID == "" {
		writeError(w, http.StatusBadRequest, "memoryId is required")
		return
	}
	logs, err := h.auditService.FindByMemoryID(r.Context(), memoryID, getLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find audit logs for memory")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// Stats handles GET /v1/audit-logs/stats
func (h *AuditHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.auditService.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get audit stats")
		return
	}

	var total int64
	for _, count := range stats {
		total += count
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"totalEvents":  total,
		"eventsByType": stats,
	})
}

