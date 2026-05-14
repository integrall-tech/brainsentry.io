package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// MemoryRepository handles memory persistence in PostgreSQL.
type MemoryRepository struct {
	pool *pgxpool.Pool
}

// NewMemoryRepository creates a new MemoryRepository.
func NewMemoryRepository(pool *pgxpool.Pool) *MemoryRepository {
	return &MemoryRepository{pool: pool}
}

const memoryColumns = `id, content, summary, category, importance, validation_status,
	embedding, metadata, source_type, source_reference, created_by, tenant_id,
	created_at, updated_at, last_accessed_at, version, access_count, injection_count,
	helpful_count, not_helpful_count, code_example, programming_language, memory_type, deleted_at,
	emotional_weight, sim_hash, valid_from, valid_to, decay_rate, superseded_by, recorded_at`

func scanMemory(row pgx.Row) (*domain.Memory, error) {
	var m domain.Memory
	err := row.Scan(
		&m.ID, &m.Content, &m.Summary, &m.Category, &m.Importance, &m.ValidationStatus,
		&m.Embedding, &m.Metadata, &m.SourceType, &m.SourceReference, &m.CreatedBy, &m.TenantID,
		&m.CreatedAt, &m.UpdatedAt, &m.LastAccessedAt, &m.Version, &m.AccessCount, &m.InjectionCount,
		&m.HelpfulCount, &m.NotHelpfulCount, &m.CodeExample, &m.ProgrammingLanguage, &m.MemoryType, &m.DeletedAt,
		&m.EmotionalWeight, &m.SimHash, &m.ValidFrom, &m.ValidTo, &m.DecayRate, &m.SupersededBy, &m.RecordedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func scanMemories(rows pgx.Rows) ([]domain.Memory, error) {
	var memories []domain.Memory
	for rows.Next() {
		var m domain.Memory
		err := rows.Scan(
			&m.ID, &m.Content, &m.Summary, &m.Category, &m.Importance, &m.ValidationStatus,
			&m.Embedding, &m.Metadata, &m.SourceType, &m.SourceReference, &m.CreatedBy, &m.TenantID,
			&m.CreatedAt, &m.UpdatedAt, &m.LastAccessedAt, &m.Version, &m.AccessCount, &m.InjectionCount,
			&m.HelpfulCount, &m.NotHelpfulCount, &m.CodeExample, &m.ProgrammingLanguage, &m.MemoryType, &m.DeletedAt,
			&m.EmotionalWeight, &m.SimHash, &m.ValidFrom, &m.ValidTo, &m.DecayRate, &m.SupersededBy, &m.RecordedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, nil
}

// Create inserts a new memory with tags.
func (r *MemoryRepository) Create(ctx context.Context, m *domain.Memory) error {
	tenantID := tenant.FromContext(ctx)
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	m.TenantID = tenantID
	if m.Version == 0 {
		m.Version = 1
	}
	if m.ValidationStatus == "" {
		m.ValidationStatus = domain.ValidationPending
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if m.RecordedAt.IsZero() {
		m.RecordedAt = now
	}
	query := fmt.Sprintf(`INSERT INTO memories (%s) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31)`, memoryColumns)

	_, err = tx.Exec(ctx, query,
		m.ID, m.Content, m.Summary, m.Category, m.Importance, m.ValidationStatus,
		m.Embedding, m.Metadata, m.SourceType, m.SourceReference, m.CreatedBy, m.TenantID,
		m.CreatedAt, m.UpdatedAt, m.LastAccessedAt, m.Version, m.AccessCount, m.InjectionCount,
		m.HelpfulCount, m.NotHelpfulCount, m.CodeExample, m.ProgrammingLanguage, m.MemoryType, m.DeletedAt,
		m.EmotionalWeight, m.SimHash, m.ValidFrom, m.ValidTo, m.DecayRate, m.SupersededBy, m.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting memory: %w", err)
	}

	if err := r.insertTags(ctx, tx, m.ID, m.Tags); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// FindByID finds a memory by ID with tenant filtering.
func (r *MemoryRepository) FindByID(ctx context.Context, id string) (*domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, memoryColumns)

	m, err := scanMemory(r.pool.QueryRow(ctx, query, id, tenantID))
	if err != nil {
		return nil, fmt.Errorf("finding memory: %w", err)
	}

	tags, err := r.loadTags(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	m.Tags = tags
	return m, nil
}

// List returns paginated memories for the current tenant.
func (r *MemoryRepository) List(ctx context.Context, page, size int) ([]domain.Memory, int64, error) {
	tenantID := tenant.FromContext(ctx)
	offset := page * size

	// Count
	var total int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting memories: %w", err)
	}

	// Fetch
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`, memoryColumns)
	rows, err := r.pool.Query(ctx, query, tenantID, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing memories: %w", err)
	}
	defer rows.Close()

	memories, err := scanMemories(rows)
	if err != nil {
		return nil, 0, err
	}

	// Load tags for each memory
	for i := range memories {
		tags, err := r.loadTags(ctx, memories[i].ID)
		if err != nil {
			return nil, 0, err
		}
		memories[i].Tags = tags
	}

	return memories, total, nil
}

// Update updates a memory.
func (r *MemoryRepository) Update(ctx context.Context, m *domain.Memory) error {
	tenantID := tenant.FromContext(ctx)
	m.UpdatedAt = time.Now()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `UPDATE memories SET content=$1, summary=$2, category=$3, importance=$4,
		validation_status=$5, embedding=$6, metadata=$7, source_type=$8, source_reference=$9,
		updated_at=$10, version=$11, code_example=$12, programming_language=$13, memory_type=$14,
		emotional_weight=$15, sim_hash=$16, valid_from=$17, valid_to=$18, decay_rate=$19, superseded_by=$20,
		access_count=$21, injection_count=$22, helpful_count=$23, not_helpful_count=$24
		WHERE id=$25 AND tenant_id=$26`

	_, err = tx.Exec(ctx, query,
		m.Content, m.Summary, m.Category, m.Importance,
		m.ValidationStatus, m.Embedding, m.Metadata, m.SourceType, m.SourceReference,
		m.UpdatedAt, m.Version, m.CodeExample, m.ProgrammingLanguage, m.MemoryType,
		m.EmotionalWeight, m.SimHash, m.ValidFrom, m.ValidTo, m.DecayRate, m.SupersededBy,
		m.AccessCount, m.InjectionCount, m.HelpfulCount, m.NotHelpfulCount,
		m.ID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("updating memory: %w", err)
	}

	// Replace tags
	_, err = tx.Exec(ctx, `DELETE FROM memory_tags WHERE memory_id = $1`, m.ID)
	if err != nil {
		return fmt.Errorf("deleting tags: %w", err)
	}
	if err := r.insertTags(ctx, tx, m.ID, m.Tags); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Delete soft-deletes a memory by setting deleted_at timestamp.
func (r *MemoryRepository) Delete(ctx context.Context, id string) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx,
		`UPDATE memories SET deleted_at = $1 WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL`,
		time.Now(), id, tenantID)
	if err != nil {
		return fmt.Errorf("soft-deleting memory: %w", err)
	}
	return nil
}

// FindByCategory returns memories filtered by category.
func (r *MemoryRepository) FindByCategory(ctx context.Context, category domain.MemoryCategory) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1 AND category = $2 AND deleted_at IS NULL ORDER BY created_at DESC`, memoryColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, string(category))
	if err != nil {
		return nil, fmt.Errorf("finding by category: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// FindByImportance returns memories filtered by importance.
func (r *MemoryRepository) FindByImportance(ctx context.Context, importance domain.ImportanceLevel) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1 AND importance = $2 AND deleted_at IS NULL ORDER BY created_at DESC`, memoryColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, string(importance))
	if err != nil {
		return nil, fmt.Errorf("finding by importance: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// FullTextSearch performs a PostgreSQL full-text search on content and summary.
func (r *MemoryRepository) FullTextSearch(ctx context.Context, query string, limit int) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)

	// Use PostgreSQL full-text search with ts_rank for relevance ordering.
	// plainto_tsquery handles user input safely (no special syntax needed).
	q := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1
		AND (to_tsvector('english', coalesce(content,'') || ' ' || coalesce(summary,'')) @@ plainto_tsquery('english', $2))
		AND deleted_at IS NULL
		AND (valid_from IS NULL OR valid_from <= NOW())
		AND (valid_to IS NULL OR valid_to > NOW())
		AND COALESCE(superseded_by, '') = ''
		ORDER BY ts_rank(to_tsvector('english', coalesce(content,'') || ' ' || coalesce(summary,'')), plainto_tsquery('english', $2)) DESC
		LIMIT $3`, memoryColumns)

	rows, err := r.pool.Query(ctx, q, tenantID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("full text search: %w", err)
	}
	defer rows.Close()

	memories, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	for i := range memories {
		tags, _ := r.loadTags(ctx, memories[i].ID)
		memories[i].Tags = tags
	}
	return memories, nil
}

// IncrementAccessCount increments the access counter and updates last accessed time.
func (r *MemoryRepository) IncrementAccessCount(ctx context.Context, id string) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx,
		`UPDATE memories SET access_count = access_count + 1, last_accessed_at = $1
		WHERE id = $2 AND tenant_id = $3`, time.Now(), id, tenantID)
	return err
}

// IncrementInjectionCount increments the injection counter.
func (r *MemoryRepository) IncrementInjectionCount(ctx context.Context, id string) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx,
		`UPDATE memories SET injection_count = injection_count + 1
		WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	return err
}

// RecordFeedback records helpful/not helpful feedback.
func (r *MemoryRepository) RecordFeedback(ctx context.Context, id string, helpful bool) error {
	tenantID := tenant.FromContext(ctx)
	var col string
	if helpful {
		col = "helpful_count"
	} else {
		col = "not_helpful_count"
	}
	_, err := r.pool.Exec(ctx,
		fmt.Sprintf(`UPDATE memories SET %s = %s + 1 WHERE id = $1 AND tenant_id = $2`, col, col),
		id, tenantID)
	return err
}

// CountByCategory returns memory counts grouped by category.
func (r *MemoryRepository) CountByCategory(ctx context.Context) (map[string]int64, error) {
	tenantID := tenant.FromContext(ctx)
	rows, err := r.pool.Query(ctx,
		`SELECT category, COUNT(*) FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL GROUP BY category`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var cat string
		var count int64
		if err := rows.Scan(&cat, &count); err != nil {
			return nil, err
		}
		result[cat] = count
	}
	return result, nil
}

// CountByImportance returns memory counts grouped by importance.
func (r *MemoryRepository) CountByImportance(ctx context.Context) (map[string]int64, error) {
	tenantID := tenant.FromContext(ctx)
	rows, err := r.pool.Query(ctx,
		`SELECT importance, COUNT(*) FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL GROUP BY importance`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var imp string
		var count int64
		if err := rows.Scan(&imp, &count); err != nil {
			return nil, err
		}
		result[imp] = count
	}
	return result, nil
}

// Count returns total memory count for tenant.
func (r *MemoryRepository) Count(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&count)
	return count, err
}

// CountActiveRecent counts memories accessed in the last 24 hours.
func (r *MemoryRepository) CountActiveRecent(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL AND last_accessed_at > $2`,
		tenantID, time.Now().Add(-24*time.Hour)).Scan(&count)
	return count, err
}

// FindAll returns all memories for the current tenant (for reprocessing).
func (r *MemoryRepository) FindAll(ctx context.Context) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`, memoryColumns)

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("finding all memories: %w", err)
	}
	defer rows.Close()

	memories, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}

	for i := range memories {
		tags, err := r.loadTags(ctx, memories[i].ID)
		if err != nil {
			return nil, err
		}
		memories[i].Tags = tags
	}

	return memories, nil
}

func (r *MemoryRepository) loadTags(ctx context.Context, memoryID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT tag FROM memory_tags WHERE memory_id = $1`, memoryID)
	if err != nil {
		return nil, fmt.Errorf("loading tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (r *MemoryRepository) insertTags(ctx context.Context, tx pgx.Tx, memoryID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("INSERT INTO memory_tags (memory_id, tag) VALUES ")
	args := make([]any, 0, len(tags)*2)
	for i, tag := range tags {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		args = append(args, memoryID, tag)
	}
	sb.WriteString(" ON CONFLICT DO NOTHING")
	_, err := tx.Exec(ctx, sb.String(), args...)
	if err != nil {
		return fmt.Errorf("inserting tags: %w", err)
	}
	return nil
}

// SaveEmbedding updates just the embedding vector for a memory.
func (r *MemoryRepository) SaveEmbedding(ctx context.Context, id string, embedding []float32) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx,
		`UPDATE memories SET embedding = $1 WHERE id = $2 AND tenant_id = $3`,
		embedding, id, tenantID)
	return err
}

// NullifyAllEmbeddings clears the embedding column on every memory across
// every tenant. Returns the number of rows touched. Intentionally NOT
// tenant-scoped — this is operator-level rebuild surface; callers must
// gate it (see internal/rebuild + trust.RequireLocalTrust).
func (r *MemoryRepository) NullifyAllEmbeddings(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE memories SET embedding = NULL WHERE embedding IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// WipeAllContextSummaries truncates the LLM-compressed context_summaries
// family across every tenant. Child tables fall via ON DELETE CASCADE.
// Returns the number of summaries removed.
func (r *MemoryRepository) WipeAllContextSummaries(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM context_summaries`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// FindByEmbeddingSimilarity finds memories by cosine similarity (PostgreSQL-based fallback).
func (r *MemoryRepository) FindByEmbeddingSimilarity(ctx context.Context, embedding []float32, limit int) ([]domain.Memory, error) {
	// This is a fallback; primary vector search should use FalkorDB.
	// PostgreSQL doesn't have native vector similarity without pgvector extension.
	// For now, fall back to full text search.
	return nil, fmt.Errorf("vector search requires FalkorDB or pgvector extension")
}

// FindByMetadata finds memories with metadata matching a key-value pair.
func (r *MemoryRepository) FindByMetadata(ctx context.Context, key, value string) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1 AND metadata->>$2 = $3 AND deleted_at IS NULL ORDER BY created_at DESC`, memoryColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, key, value)
	if err != nil {
		return nil, fmt.Errorf("finding by metadata: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// FindBySimHash returns memories with a non-empty sim_hash for the tenant (for dedup checking).
func (r *MemoryRepository) FindSimHashes(ctx context.Context) (map[string]string, error) {
	tenantID := tenant.FromContext(ctx)
	rows, err := r.pool.Query(ctx,
		`SELECT id, sim_hash FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL AND sim_hash != ''`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("finding sim hashes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		result[id] = hash
	}
	return result, nil
}

// BoostAccessCount adds to the access count (used for dedup boost).
func (r *MemoryRepository) BoostAccessCount(ctx context.Context, id string, boost int) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx,
		`UPDATE memories SET access_count = access_count + $1, last_accessed_at = $2
		WHERE id = $3 AND tenant_id = $4`, boost, time.Now(), id, tenantID)
	return err
}

// SupersedeMemory marks an existing memory as superseded by a new one.
func (r *MemoryRepository) SupersedeMemory(ctx context.Context, oldID, newID string) error {
	tenantID := tenant.FromContext(ctx)
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE memories SET superseded_by = $1, valid_to = $2, updated_at = $3
		WHERE id = $4 AND tenant_id = $5 AND deleted_at IS NULL`,
		newID, now, now, oldID, tenantID)
	if err != nil {
		return fmt.Errorf("superseding memory: %w", err)
	}
	return nil
}

// ExpireStaleMemories soft-deletes memories past their valid_to date.
func (r *MemoryRepository) ExpireStaleMemories(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	now := time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE memories SET deleted_at = $1
		WHERE tenant_id = $2 AND valid_to IS NOT NULL AND valid_to < $3 AND deleted_at IS NULL`,
		now, tenantID, now)
	if err != nil {
		return 0, fmt.Errorf("expiring stale memories: %w", err)
	}
	return tag.RowsAffected(), nil
}

// FindActiveMemories returns memories that are currently valid (within valid_from/valid_to range).
func (r *MemoryRepository) FindActiveMemories(ctx context.Context, limit int) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	now := time.Now()
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE tenant_id = $1 AND deleted_at IS NULL
		AND (valid_from IS NULL OR valid_from <= $2)
		AND (valid_to IS NULL OR valid_to > $2)
		AND superseded_by = ''
		ORDER BY created_at DESC LIMIT $3`, memoryColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, now, limit)
	if err != nil {
		return nil, fmt.Errorf("finding active memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// FindByRecordedRange returns memories recorded in the system between from
// (inclusive) and to (inclusive). Used by the bi-temporal timeline view.
func (r *MemoryRepository) FindByRecordedRange(ctx context.Context, from, to time.Time, limit int) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	if limit <= 0 {
		limit = 200
	}
	if to.IsZero() {
		to = time.Now()
	}
	query := fmt.Sprintf(`SELECT %s FROM memories
		WHERE tenant_id = $1
		  AND deleted_at IS NULL
		  AND recorded_at <= $2
		  AND ($3::timestamptz IS NULL OR recorded_at >= $3)
		ORDER BY recorded_at DESC
		LIMIT $4`, memoryColumns)
	var fromArg any = nil
	if !from.IsZero() {
		fromArg = from
	}
	rows, err := r.pool.Query(ctx, query, tenantID, to, fromArg, limit)
	if err != nil {
		return nil, fmt.Errorf("finding memories by recorded range: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// FindAsOf returns memories valid at a specific point in time and recorded
// in the system no later than that point. Implements bi-temporal "time travel"
// queries — how the system saw the world at instant `asOf`.
func (r *MemoryRepository) FindAsOf(ctx context.Context, asOf time.Time, limit int) ([]domain.Memory, error) {
	tenantID := tenant.FromContext(ctx)
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf(`SELECT %s FROM memories
		WHERE tenant_id = $1
		  AND deleted_at IS NULL
		  AND recorded_at <= $2
		  AND (valid_from IS NULL OR valid_from <= $2)
		  AND (valid_to IS NULL OR valid_to > $2)
		ORDER BY recorded_at DESC
		LIMIT $3`, memoryColumns)
	rows, err := r.pool.Query(ctx, query, tenantID, asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("finding as_of memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// MemoryToJSON converts metadata to JSON for storage.
func MemoryToJSON(data map[string]any) json.RawMessage {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	return b
}
