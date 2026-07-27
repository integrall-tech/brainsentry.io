package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// defaultMaxPerRun bounds a retention sweep when the tenant policy does not.
// A first run on an old tenant could otherwise try to delete years of data in
// one transaction.
const defaultMaxPerRun = 1000

// purgeRepository is the Postgres side of erasure.
type purgeRepository interface {
	ResolveScope(ctx context.Context, scope postgres.PurgeScope) ([]string, error)
	PurgeMemories(ctx context.Context, ids []string, dryRun bool) (postgres.PurgeCounts, error)
	RedactAuditContent(ctx context.Context, ids []string, dryRun bool) (int64, error)
	FindExpiredBefore(ctx context.Context, cutoff time.Time, limit int) ([]string, error)
}

// graphPurger is the FalkorDB side. Optional: when FalkorDB is down the
// erasure still runs on Postgres and says so in the result, rather than
// failing and leaving the caller unsure whether anything was removed.
type graphPurger interface {
	PurgeMemoryNodes(ctx context.Context, tenantID string, ids []string) error
}

// tenantReader supplies the per-tenant policy.
type tenantReader interface {
	FindByID(ctx context.Context, id string) (*domain.Tenant, error)
}

// receiptWriter persists the proof.
type receiptWriter interface {
	Create(ctx context.Context, r *domain.ErasureReceipt) error
}

// RetentionService executes retention policy and data-subject erasure.
type RetentionService struct {
	repo     purgeRepository
	graph    graphPurger
	tenants  tenantReader
	receipts receiptWriter
}

// NewRetentionService creates a new RetentionService.
func NewRetentionService(repo purgeRepository, graph graphPurger, tenants tenantReader, receipts receiptWriter) *RetentionService {
	return &RetentionService{repo: repo, graph: graph, tenants: tenants, receipts: receipts}
}

// ErasureResult is what both operations return.
type ErasureResult struct {
	ReceiptID string               `json:"receiptId"`
	Kind      string               `json:"kind"`
	Executed  bool                 `json:"executed"`
	Matched   int                  `json:"matchedMemories"`
	Counts    postgres.PurgeCounts `json:"counts"`
	Redacted  int64                `json:"auditRowsRedacted"`
	// GraphPurged is false when FalkorDB could not be reached; the Postgres
	// side still happened, and the caller needs to know the difference.
	GraphPurged bool   `json:"graphPurged"`
	GraphError  string `json:"graphError,omitempty"`
}

// EraseSubject removes every trace of a data subject within the caller's
// tenant (RFC-014 §10).
//
// confirm must be true to actually delete. The default is a dry run that
// reports exactly what would go — same discipline as the rebuild command,
// which defaults to a plan precisely because a destructive default is a
// mistake you only notice afterwards.
func (s *RetentionService) EraseSubject(ctx context.Context, scope postgres.PurgeScope, reason, requestedBy string, confirm bool) (*ErasureResult, error) {
	if scope.IsZero() {
		return nil, fmt.Errorf("scope is required: refusing to erase every memory of the tenant")
	}
	if reason == "" {
		// The receipt exists to be shown later; a receipt with no reason
		// cannot answer why the removal happened.
		return nil, fmt.Errorf("reason is required")
	}
	return s.run(ctx, "erasure", scope, nil, reason, requestedBy, confirm)
}

// RunRetention applies the tenant's declared policy: purge memories whose
// validity window closed more than N days ago.
//
// A tenant with no policy is a no-op, not an error — that is every tenant
// until someone decides the number.
func (s *RetentionService) RunRetention(ctx context.Context, requestedBy string, confirm bool) (*ErasureResult, error) {
	tenantID := tenant.FromContext(ctx)
	t, err := s.tenants.FindByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("reading tenant policy: %w", err)
	}

	policy := domain.RetentionPolicyFromSettings(t.Settings)
	if !policy.Enabled() {
		return &ErasureResult{
			Kind:     "retention",
			Executed: false,
			Counts:   postgres.PurgeCounts{},
		}, nil
	}

	limit := policy.MaxPerRun
	if limit <= 0 {
		limit = defaultMaxPerRun
	}
	cutoff := policy.CutoffFrom(time.Now())

	ids, err := s.repo.FindExpiredBefore(ctx, cutoff, limit)
	if err != nil {
		return nil, err
	}

	reason := fmt.Sprintf("retention policy: valid_to older than %d days (cutoff %s)",
		policy.PurgeAfterValidToDays, cutoff.Format(time.RFC3339))
	return s.run(ctx, "retention", postgres.PurgeScope{}, ids, reason, requestedBy, confirm)
}

// run is the shared engine. preResolved short-circuits scope resolution for
// the retention path, which already knows its ids.
func (s *RetentionService) run(ctx context.Context, kind string, scope postgres.PurgeScope, preResolved []string, reason, requestedBy string, confirm bool) (*ErasureResult, error) {
	tenantID := tenant.FromContext(ctx)

	ids := preResolved
	if ids == nil {
		var err error
		ids, err = s.repo.ResolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
	}

	result := &ErasureResult{
		ReceiptID: uuid.NewString(),
		Kind:      kind,
		Executed:  confirm,
		Matched:   len(ids),
		Counts:    postgres.PurgeCounts{},
	}

	if len(ids) == 0 {
		// Nothing matched. Still receipted: "we looked and found nothing" is
		// an answer a data subject may need documented.
		s.writeReceipt(ctx, result, tenantID, scope, reason, requestedBy)
		return result, nil
	}

	// ORDER MATTERS, and getting it wrong is silent.
	//
	// RedactAuditContent finds its audit_logs rows by joining through the
	// audit_memories_* link tables — which PurgeMemories deletes. Purging
	// first therefore leaves the subject's text sitting in
	// audit_logs.user_request/reasoning with nothing left to point at it.
	// Verified against a real schema before this was reordered: the purge
	// completed, and the audit row still read "o que o acme disse sobre
	// precos".
	//
	// If redaction succeeds and the purge then fails, the audit loses content
	// for memories that still exist — a degradation, not a leak. That is the
	// direction to fail in.
	redacted, err := s.repo.RedactAuditContent(ctx, ids, !confirm)
	if err != nil {
		return nil, err
	}
	result.Redacted = redacted

	counts, err := s.repo.PurgeMemories(ctx, ids, !confirm)
	if err != nil {
		return nil, err
	}
	result.Counts = counts

	// The graph carries content too (SaveToGraph copies content/summary onto
	// the node), so it is part of the erasure, not an afterthought.
	if s.graph != nil && confirm {
		if err := s.graph.PurgeMemoryNodes(ctx, tenantID, ids); err != nil {
			// Deliberately not fatal: the Postgres erasure already committed,
			// and failing here would report "erasure failed" for something
			// that mostly succeeded. The caller is told exactly what is left.
			slog.Error("graph purge failed during erasure",
				"receiptId", result.ReceiptID, "tenantId", tenantID, "error", err)
			result.GraphError = err.Error()
		} else {
			result.GraphPurged = true
		}
	}

	s.writeReceipt(ctx, result, tenantID, scope, reason, requestedBy)
	return result, nil
}

func (s *RetentionService) writeReceipt(ctx context.Context, result *ErasureResult, tenantID string, scope postgres.PurgeScope, reason, requestedBy string) {
	if s.receipts == nil {
		return
	}

	scopeJSON, _ := json.Marshal(map[string]any{
		"tag":             scope.Tag,
		"sourceReference": scope.SourceReference,
		"memoryIdCount":   len(scope.MemoryIDs),
	})
	countsJSON, _ := json.Marshal(result.Counts)

	receipt := &domain.ErasureReceipt{
		ID:          result.ReceiptID,
		TenantID:    tenantID,
		Kind:        result.Kind,
		Scope:       scopeJSON,
		Counts:      countsJSON,
		Reason:      reason,
		RequestedBy: requestedBy,
		Executed:    result.Executed,
		CreatedAt:   time.Now(),
	}
	if err := s.receipts.Create(ctx, receipt); err != nil {
		// The data is already gone; losing the receipt is bad but must not
		// look like the erasure failed.
		slog.Error("could not persist erasure receipt",
			"receiptId", result.ReceiptID, "tenantId", tenantID, "error", err)
	}
}
