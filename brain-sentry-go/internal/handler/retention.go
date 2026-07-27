package handler

import (
	"encoding/json"
	"net/http"

	"github.com/integraltech/brainsentry/internal/middleware"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/internal/service"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// RetentionHandler exposes retention sweeps and data-subject erasure.
//
// Both are reachable by the tenant's service key: the Core is who receives
// the customer's removal request, so making this admin-only would turn a
// legal deadline into a manual ticket. The key is already pinned to one
// tenant, so it can only ever erase its own data.
type RetentionHandler struct {
	retention *service.RetentionService
	receipts  *postgres.ReceiptRepository
}

// NewRetentionHandler creates a new RetentionHandler.
func NewRetentionHandler(retention *service.RetentionService, receipts *postgres.ReceiptRepository) *RetentionHandler {
	return &RetentionHandler{retention: retention, receipts: receipts}
}

type erasureRequest struct {
	// Scope. Tag is the data-subject selector for VendaX ("cliente:{ref}").
	Tag             string   `json:"tag,omitempty"`
	MemoryIds       []string `json:"memoryIds,omitempty"`
	SourceReference string   `json:"sourceReference,omitempty"`

	Reason string `json:"reason"`
	// Confirm must be true to actually delete. Absent = dry run, which
	// reports exactly what would go. A destructive default is a mistake you
	// only find out about afterwards.
	Confirm bool `json:"confirm,omitempty"`
}

// Erase handles POST /v1/privacy/erasure
//
//	@Summary		Erase every trace of a data subject
//	@Description	Removes memories and ALL derived copies — versions, relationships, tags, note/hindsight links, graph nodes — and strips subject content from audit rows while keeping the audit event itself. Defaults to a dry run; set confirm=true to execute. Always produces a receipt.
//	@Tags			Privacy
//	@Accept			json
//	@Produce		json
//	@Param			request	body		erasureRequest	true	"Scope, reason and confirm"
//	@Success		200		{object}	service.ErasureResult
//	@Failure		400		{object}	dto.ErrorResponse
//	@Security		BearerAuth
//	@Security		ServiceKeyAuth
//	@Router			/v1/privacy/erasure [post]
func (h *RetentionHandler) Erase(w http.ResponseWriter, r *http.Request) {
	var req erasureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	requestedBy := requesterID(r)

	result, err := h.retention.EraseSubject(r.Context(), postgres.PurgeScope{
		Tag:             req.Tag,
		MemoryIDs:       req.MemoryIds,
		SourceReference: req.SourceReference,
	}, req.Reason, requestedBy, req.Confirm)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type retentionRunRequest struct {
	Confirm bool `json:"confirm,omitempty"`
}

// RunRetention handles POST /v1/retention/run
//
//	@Summary		Apply the tenant's retention policy
//	@Description	Purges memories whose validity window closed more than the tenant's configured grace period ago. The policy lives in tenants.settings.retention; a tenant without one is a no-op, never a default deletion. Defaults to a dry run.
//	@Tags			Privacy
//	@Accept			json
//	@Produce		json
//	@Param			request	body		retentionRunRequest	false	"confirm"
//	@Success		200		{object}	service.ErasureResult
//	@Security		BearerAuth
//	@Security		ServiceKeyAuth
//	@Router			/v1/retention/run [post]
func (h *RetentionHandler) RunRetention(w http.ResponseWriter, r *http.Request) {
	var req retentionRunRequest
	// An empty body is a valid dry run.
	_ = json.NewDecoder(r.Body).Decode(&req)

	result, err := h.retention.RunRetention(r.Context(), requesterID(r), req.Confirm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListReceipts handles GET /v1/privacy/receipts
//
//	@Summary		List erasure and retention receipts
//	@Description	The audit surface for "prove this was removed" — identifiers, counts and timing, never subject content.
//	@Tags			Privacy
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Security		BearerAuth
//	@Security		ServiceKeyAuth
//	@Router			/v1/privacy/receipts [get]
func (h *RetentionHandler) ListReceipts(w http.ResponseWriter, r *http.Request) {
	receipts, err := h.receipts.ListByTenant(r.Context(), tenant.FromContext(r.Context()), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list receipts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"receipts": receipts, "total": len(receipts)})
}

// requesterID identifies who asked, for the receipt. A service key is
// recorded as its key id: "which integration did this" is the question that
// matters when the caller is not a person.
func requesterID(r *http.Request) string {
	if p := middleware.ServicePrincipalFromContext(r.Context()); p != nil {
		return "service:" + p.KeyID
	}
	if claims := middleware.ClaimsFromContext(r.Context()); claims != nil {
		return claims.UserID
	}
	return ""
}
