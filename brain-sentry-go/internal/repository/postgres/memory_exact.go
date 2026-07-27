package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// ExactFilter selects memories by identity rather than by similarity.
//
// The audit routine (RFC-014 §9.2) asks "give me the fact produced by event
// X" and then compares it against its source. That is a lookup, not a
// search: ranking a known key by cosine distance would be the wrong tool,
// would cost an embedding call, and could silently return a *different*
// memory that happens to be close in vector space.
type ExactFilter struct {
	// SourceReference is the domain event that produced the memory
	// ("decisao:{id}", "cotacao:{id}"). Matched by equality.
	SourceReference string
	// Metadata is matched by JSONB containment: every pair given must be
	// present, extra keys on the memory are fine.
	Metadata map[string]string
	Limit    int
}

// IsZero reports whether the filter selects nothing in particular — which is
// what tells the search path to stay on the semantic route.
func (f ExactFilter) IsZero() bool {
	return f.SourceReference == "" && len(f.Metadata) == 0
}

// FindByExactFilter returns memories matching the filter, newest first.
//
// Always tenant-scoped, and always excludes soft-deleted rows. Unlike the
// semantic path it does NOT filter out expired or superseded memories: the
// audit needs to see the fact it is auditing even when that fact is already
// dead, otherwise "was this revoked?" is unanswerable.
func (r *MemoryRepository) FindByExactFilter(ctx context.Context, f ExactFilter) ([]domain.Memory, error) {
	if f.IsZero() {
		return nil, fmt.Errorf("exact filter requires sourceReference or metadata")
	}

	tenantID := tenant.FromContext(ctx)
	args := []any{tenantID}
	var where strings.Builder
	where.WriteString("tenant_id = $1 AND deleted_at IS NULL")

	if f.SourceReference != "" {
		args = append(args, f.SourceReference)
		fmt.Fprintf(&where, " AND source_reference = $%d", len(args))
	}

	if len(f.Metadata) > 0 {
		// One containment check for the whole map: @> is a single index
		// probe, whereas a chain of ->> comparisons is one filter per key.
		payload, err := json.Marshal(f.Metadata)
		if err != nil {
			return nil, fmt.Errorf("encoding metadata filter: %w", err)
		}
		args = append(args, string(payload))
		fmt.Fprintf(&where, " AND metadata @> $%d::jsonb", len(args))
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)

	query := fmt.Sprintf(`SELECT %s FROM memories WHERE %s ORDER BY created_at DESC LIMIT $%d`,
		memoryColumns, where.String(), len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exact search: %w", err)
	}
	defer rows.Close()

	return scanMemories(rows)
}

// BatchExpireResult reports what a batch expiry actually did.
type BatchExpireResult struct {
	Expired int64    `json:"expired"`
	IDs     []string `json:"ids"`
}

// BatchExpire closes the validity window of many memories in ONE transaction.
//
// The audit revokes in bulk (a reverted decision invalidates every fact
// derived from it); doing that one call at a time multiplies round-trips and,
// worse, can leave the set half-revoked if the caller dies midway.
//
// Sets valid_to = NOW() rather than deleting: the fact WAS true, and
// bi-temporality means "no longer valid" is different from "never happened".
// /v1/memories/as-of still answers correctly about the past.
func (r *MemoryRepository) BatchExpire(ctx context.Context, ids []string, sourceReference, reason string) (*BatchExpireResult, error) {
	if len(ids) == 0 && sourceReference == "" {
		return nil, fmt.Errorf("batch expire requires ids or sourceReference")
	}

	tenantID := tenant.FromContext(ctx)

	// Tenant is in the WHERE clause, not merely checked beforehand: a bulk
	// write is exactly where a missing scope would be most expensive.
	args := []any{tenantID}
	var where strings.Builder
	where.WriteString(`tenant_id = $1 AND deleted_at IS NULL
		AND (valid_to IS NULL OR valid_to > NOW())`)

	if len(ids) > 0 {
		args = append(args, ids)
		fmt.Fprintf(&where, " AND id = ANY($%d)", len(args))
	}
	if sourceReference != "" {
		args = append(args, sourceReference)
		fmt.Fprintf(&where, " AND source_reference = $%d", len(args))
	}

	// The reason rides in metadata so the revocation explains itself where
	// the memory lives. jsonb_build_object keeps it a proper object even when
	// metadata was NULL.
	args = append(args, reason)
	query := fmt.Sprintf(`UPDATE memories
		SET valid_to = NOW(),
		    updated_at = NOW(),
		    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('expiredReason', $%d::text, 'expiredAt', NOW())
		WHERE %s
		RETURNING id`, len(args), where.String())

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch expire: %w", err)
	}
	defer rows.Close()

	result := &BatchExpireResult{IDs: []string{}}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning expired id: %w", err)
		}
		result.IDs = append(result.IDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.Expired = int64(len(result.IDs))
	return result, nil
}
