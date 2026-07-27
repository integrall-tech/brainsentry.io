package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/integraltech/brainsentry/pkg/tenant"
)

// PurgeScope selects what to erase. Exactly one of the selectors is enough;
// combining them narrows.
//
// Tag is the VendaX case: every fact about one customer carries
// "cliente:{ref}" (RFC-014 §6.1), so the tag IS the data subject.
type PurgeScope struct {
	Tag             string
	MemoryIDs       []string
	SourceReference string
}

// IsZero reports whether the scope selects nothing. A purge with an empty
// scope must never run: it would match every memory in the tenant.
func (s PurgeScope) IsZero() bool {
	return s.Tag == "" && len(s.MemoryIDs) == 0 && s.SourceReference == ""
}

// PurgeCounts reports rows affected per surface.
//
// Per-surface rather than a single total on purpose: "12 rows deleted" cannot
// tell you whether memory_versions was included, and that is precisely the
// surface an incomplete purge leaves behind.
type PurgeCounts map[string]int64

// Total sums every surface.
func (c PurgeCounts) Total() int64 {
	var n int64
	for _, v := range c {
		n += v
	}
	return n
}

// ResolveScope turns a scope into the concrete memory ids it selects, always
// within the caller's tenant.
//
// Soft-deleted memories are INCLUDED: a memory the app already "deleted" still
// has its content in the row, so an erasure that skipped it would leave the
// subject's text in the database.
func (r *MemoryRepository) ResolveScope(ctx context.Context, scope PurgeScope) ([]string, error) {
	if scope.IsZero() {
		return nil, fmt.Errorf("purge scope is empty: refusing to match every memory")
	}

	tenantID := tenant.FromContext(ctx)
	args := []any{tenantID}
	where := "m.tenant_id = $1"

	if scope.Tag != "" {
		args = append(args, scope.Tag)
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM memory_tags mt WHERE mt.memory_id = m.id AND mt.tag = $%d)`, len(args))
	}
	if scope.SourceReference != "" {
		args = append(args, scope.SourceReference)
		where += fmt.Sprintf(" AND m.source_reference = $%d", len(args))
	}
	if len(scope.MemoryIDs) > 0 {
		args = append(args, scope.MemoryIDs)
		where += fmt.Sprintf(" AND m.id = ANY($%d)", len(args))
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT m.id FROM memories m WHERE %s`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("resolving purge scope: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PurgeMemories permanently removes memories and everything derived from
// them, in one transaction.
//
// WHY THIS IS NOT A DELETE STATEMENT: only memory_tags declares
// ON DELETE CASCADE. memory_versions, memory_relationships, the
// audit_memories_* link tables and the note/hindsight link tables all hold
// memory_id as a plain column. A `DELETE FROM memories` therefore leaves
// memory_versions holding the FULL previous content of every purged memory —
// an erasure that erases nothing. Each surface below is deleted explicitly
// for that reason; adding a new table that references memories means adding
// it here too.
//
// Audit rows are handled differently, per the retention decision: the event
// survives (who, when, which id) and only its content-bearing columns are
// nulled. Deleting the trail would destroy the evidence that the erasure
// itself happened.
//
// dryRun runs the identical statements inside a transaction that is rolled
// back, so the counts are what the destructive run would really do — not an
// estimate from a separate COUNT query that could disagree.
func (r *MemoryRepository) PurgeMemories(ctx context.Context, ids []string, dryRun bool) (PurgeCounts, error) {
	counts := PurgeCounts{}
	if len(ids) == 0 {
		return counts, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning purge: %w", err)
	}
	// Rollback is a no-op after a successful Commit.
	defer func() { _ = tx.Rollback(ctx) }()

	exec := func(surface, sql string, args ...any) error {
		tag, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return fmt.Errorf("purging %s: %w", surface, err)
		}
		counts[surface] = tag.RowsAffected()
		return nil
	}

	// Children first: relationships and links would otherwise reference rows
	// that no longer exist, and the link tables have no FK to clean them up.
	steps := []struct {
		surface string
		sql     string
	}{
		{"memory_versions", `DELETE FROM memory_versions WHERE memory_id = ANY($1)`},
		{"memory_relationships", `DELETE FROM memory_relationships WHERE from_memory_id = ANY($1) OR to_memory_id = ANY($1)`},
		{"memory_tags", `DELETE FROM memory_tags WHERE memory_id = ANY($1)`},
		{"note_related_memories", `DELETE FROM note_related_memories WHERE memory_id = ANY($1)`},
		{"hindsight_related_memories", `DELETE FROM hindsight_related_memories WHERE memory_id = ANY($1)`},
		{"audit_memories_accessed", `DELETE FROM audit_memories_accessed WHERE memory_id = ANY($1)`},
		{"audit_memories_created", `DELETE FROM audit_memories_created WHERE memory_id = ANY($1)`},
		{"audit_memories_modified", `DELETE FROM audit_memories_modified WHERE memory_id = ANY($1)`},
	}
	for _, s := range steps {
		if err := exec(s.surface, s.sql, ids); err != nil {
			return nil, err
		}
	}

	// The memory rows themselves, tenant-scoped as a second barrier: the ids
	// came from ResolveScope, which is already tenant-scoped, but a bulk
	// delete is where a lost scope costs the most.
	if err := exec("memories",
		`DELETE FROM memories WHERE id = ANY($1) AND tenant_id = $2`,
		ids, tenant.FromContext(ctx)); err != nil {
		return nil, err
	}

	if dryRun {
		// Roll back explicitly and report what would have happened.
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			return nil, fmt.Errorf("rolling back dry run: %w", err)
		}
		return counts, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing purge: %w", err)
	}
	return counts, nil
}

// RedactAuditContent strips subject content from audit rows that reference
// the purged memories, keeping the event itself.
//
// audit_logs carries user_request, reasoning, decision and input_data — free
// text and JSON that can quote the subject. Nulling those columns keeps the
// answer to "what happened to this data" available without keeping the data.
func (r *MemoryRepository) RedactAuditContent(ctx context.Context, ids []string, dryRun bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	const sql = `UPDATE audit_logs SET
			user_request = NULL,
			reasoning    = NULL,
			decision     = NULL,
			input_data   = NULL
		WHERE id IN (
			SELECT audit_log_id FROM audit_memories_accessed WHERE memory_id = ANY($1)
			UNION SELECT audit_log_id FROM audit_memories_created  WHERE memory_id = ANY($1)
			UNION SELECT audit_log_id FROM audit_memories_modified WHERE memory_id = ANY($1)
		)`

	if dryRun {
		// Counting the target set is exact here — unlike the purge, this is a
		// single statement with no ordering effects.
		var n int64
		err := r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE id IN (
			SELECT audit_log_id FROM audit_memories_accessed WHERE memory_id = ANY($1)
			UNION SELECT audit_log_id FROM audit_memories_created  WHERE memory_id = ANY($1)
			UNION SELECT audit_log_id FROM audit_memories_modified WHERE memory_id = ANY($1))`, ids).Scan(&n)
		if err != nil {
			return 0, fmt.Errorf("counting audit rows to redact: %w", err)
		}
		return n, nil
	}

	tag, err := r.pool.Exec(ctx, sql, ids)
	if err != nil {
		return 0, fmt.Errorf("redacting audit content: %w", err)
	}
	return tag.RowsAffected(), nil
}

// FindExpiredBefore returns memories whose validity window closed before the
// cutoff — the input to a retention sweep.
//
// Only rows with a valid_to in the past qualify: a memory with no valid_to
// makes no claim about when it stops being true, and purging it on age alone
// would delete exactly the structural preferences (RFC-014 §9.1) that are
// supposed to have no deadline.
func (r *MemoryRepository) FindExpiredBefore(ctx context.Context, cutoff time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM memories
		 WHERE tenant_id = $1 AND valid_to IS NOT NULL AND valid_to < $2
		 ORDER BY valid_to ASC LIMIT $3`,
		tenant.FromContext(ctx), cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("finding expired memories: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
