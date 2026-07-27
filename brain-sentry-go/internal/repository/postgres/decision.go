package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// DecisionRepository persists Decision records.
type DecisionRepository struct {
	pool *pgxpool.Pool
}

// NewDecisionRepository constructs the repository.
func NewDecisionRepository(pool *pgxpool.Pool) *DecisionRepository {
	return &DecisionRepository{pool: pool}
}

// decisionColumns is the INSERT column list — bare names, no casts.
const decisionColumns = `id, tenant_id, category, scenario, reasoning, outcome, confidence,
	agent_id, session_id, parent_decision_id, entity_ids, memory_ids, policy_violations,
	embedding, metadata, created_at, valid_from, valid_until, recorded_at, superseded_by`

// decisionSelectColumns is the SELECT projection. It differs from
// decisionColumns in one place: embedding is cast back to float4[], because
// pgx cannot scan pgvector's text output into []float32. See vector.go.
var decisionSelectColumns = `id, tenant_id, category, scenario, reasoning, outcome, confidence,
	agent_id, session_id, parent_decision_id, entity_ids, memory_ids, policy_violations,
	` + vectorSelect("embedding") + `, metadata, created_at, valid_from, valid_until, recorded_at, superseded_by`

func scanDecision(row pgx.Row) (*domain.Decision, error) {
	var d domain.Decision
	var parent *string
	var superseded *string
	var entityIDs, memoryIDs, violations []byte
	err := row.Scan(
		&d.ID, &d.TenantID, &d.Category, &d.Scenario, &d.Reasoning, &d.Outcome, &d.Confidence,
		&d.AgentID, &d.SessionID, &parent, &entityIDs, &memoryIDs, &violations,
		&d.Embedding, &d.Metadata, &d.CreatedAt, &d.ValidFrom, &d.ValidUntil, &d.RecordedAt, &superseded,
	)
	if err != nil {
		return nil, err
	}
	if parent != nil {
		d.ParentDecisionID = *parent
	}
	if superseded != nil {
		d.SupersededBy = *superseded
	}
	_ = json.Unmarshal(entityIDs, &d.EntityIDs)
	_ = json.Unmarshal(memoryIDs, &d.MemoryIDs)
	_ = json.Unmarshal(violations, &d.PolicyViolations)
	return &d, nil
}

// Create inserts a new Decision.
func (r *DecisionRepository) Create(ctx context.Context, d *domain.Decision) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if d.TenantID == "" {
		d.TenantID = tenant.FromContext(ctx)
	}
	if d.Outcome == "" {
		d.Outcome = domain.DecisionPending
	}
	now := time.Now()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.RecordedAt.IsZero() {
		d.RecordedAt = now
	}

	entityIDs, _ := json.Marshal(coalesceStrings(d.EntityIDs))
	memoryIDs, _ := json.Marshal(coalesceStrings(d.MemoryIDs))
	violations, _ := json.Marshal(coalesceStrings(d.PolicyViolations))
	metadata := d.Metadata
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}

	var parent *string
	if d.ParentDecisionID != "" {
		parent = &d.ParentDecisionID
	}
	var superseded *string
	if d.SupersededBy != "" {
		superseded = &d.SupersededBy
	}

	// $14::vector — the embedding travels as a pgvector text literal; without
	// the cast Postgres receives "{...}" and rejects it. See vector.go.
	query := fmt.Sprintf(`INSERT INTO decisions (%s)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::vector,$15,$16,$17,$18,$19,$20)`, decisionColumns)

	_, err := r.pool.Exec(ctx, query,
		d.ID, d.TenantID, d.Category, d.Scenario, d.Reasoning, d.Outcome, d.Confidence,
		d.AgentID, d.SessionID, parent, entityIDs, memoryIDs, violations,
		vectorParam(d.Embedding), metadata, d.CreatedAt, d.ValidFrom, d.ValidUntil, d.RecordedAt, superseded,
	)
	if err != nil {
		return fmt.Errorf("inserting decision: %w", err)
	}
	return nil
}

// FindByID returns a single decision scoped to tenant.
func (r *DecisionRepository) FindByID(ctx context.Context, id string) (*domain.Decision, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM decisions WHERE id = $1 AND tenant_id = $2`, decisionSelectColumns)
	d, err := scanDecision(r.pool.QueryRow(ctx, query, id, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("decision not found: %s", id)
		}
		return nil, err
	}
	return d, nil
}

// DecisionFilter narrows listing queries.
type DecisionFilter struct {
	Category  string
	AgentID   string
	SessionID string
	Outcome   domain.DecisionOutcome
	AsOf      *time.Time
	Limit     int
	Offset    int
}

// List returns decisions for the tenant, newest first.
func (r *DecisionRepository) List(ctx context.Context, f DecisionFilter) ([]*domain.Decision, error) {
	tenantID := tenant.FromContext(ctx)
	args := []any{tenantID}
	where := "tenant_id = $1"
	if f.Category != "" {
		args = append(args, f.Category)
		where += fmt.Sprintf(" AND category = $%d", len(args))
	}
	if f.AgentID != "" {
		args = append(args, f.AgentID)
		where += fmt.Sprintf(" AND agent_id = $%d", len(args))
	}
	if f.SessionID != "" {
		args = append(args, f.SessionID)
		where += fmt.Sprintf(" AND session_id = $%d", len(args))
	}
	if f.Outcome != "" {
		args = append(args, f.Outcome)
		where += fmt.Sprintf(" AND outcome = $%d", len(args))
	}
	if f.AsOf != nil {
		args = append(args, *f.AsOf)
		idx := len(args)
		where += fmt.Sprintf(" AND (valid_from IS NULL OR valid_from <= $%d) AND (valid_until IS NULL OR valid_until > $%d)", idx, idx)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf(`SELECT %s FROM decisions WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		decisionSelectColumns, where, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Decision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning decision: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Children returns decisions whose ParentDecisionID points at id.
func (r *DecisionRepository) Children(ctx context.Context, id string) ([]*domain.Decision, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM decisions WHERE tenant_id = $1 AND parent_decision_id = $2 ORDER BY created_at ASC`, decisionSelectColumns)
	rows, err := r.pool.Query(ctx, query, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Decision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// FindPrecedents returns nearest decisions in the same category ordered by
// cosine similarity against the provided embedding. Falls back to created_at
// DESC when the embedding is empty.
func (r *DecisionRepository) FindPrecedents(ctx context.Context, category string, embedding []float32, limit int) ([]*domain.DecisionPrecedent, error) {
	tenantID := tenant.FromContext(ctx)
	if limit <= 0 {
		limit = 5
	}

	if len(embedding) == 0 {
		query := fmt.Sprintf(`SELECT %s FROM decisions
			WHERE tenant_id = $1 AND category = $2
			ORDER BY created_at DESC LIMIT $3`, decisionSelectColumns)
		rows, err := r.pool.Query(ctx, query, tenantID, category, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []*domain.DecisionPrecedent
		for rows.Next() {
			d, err := scanDecision(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, &domain.DecisionPrecedent{Decision: d, Similarity: 0})
		}
		return out, rows.Err()
	}

	query := fmt.Sprintf(`SELECT %s, 1 - (embedding <=> $3::vector) AS similarity
		FROM decisions
		WHERE tenant_id = $1 AND category = $2 AND embedding IS NOT NULL
		ORDER BY embedding <=> $3::vector ASC
		LIMIT $4`, decisionSelectColumns)

	// Same encoding rule as the INSERT: the $3::vector cast only works on a
	// pgvector text literal, not on pgx's "{...}" array rendering.
	rows, err := r.pool.Query(ctx, query, tenantID, category, vectorParam(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.DecisionPrecedent
	for rows.Next() {
		var d domain.Decision
		var parent, superseded *string
		var entityIDs, memoryIDs, violations []byte
		var similarity float64
		err := rows.Scan(
			&d.ID, &d.TenantID, &d.Category, &d.Scenario, &d.Reasoning, &d.Outcome, &d.Confidence,
			&d.AgentID, &d.SessionID, &parent, &entityIDs, &memoryIDs, &violations,
			&d.Embedding, &d.Metadata, &d.CreatedAt, &d.ValidFrom, &d.ValidUntil, &d.RecordedAt, &superseded,
			&similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning precedent: %w", err)
		}
		if parent != nil {
			d.ParentDecisionID = *parent
		}
		if superseded != nil {
			d.SupersededBy = *superseded
		}
		_ = json.Unmarshal(entityIDs, &d.EntityIDs)
		_ = json.Unmarshal(memoryIDs, &d.MemoryIDs)
		_ = json.Unmarshal(violations, &d.PolicyViolations)
		out = append(out, &domain.DecisionPrecedent{Decision: &d, Similarity: similarity})
	}
	return out, rows.Err()
}

// Supersede marks a decision as superseded by a newer one.
func (r *DecisionRepository) Supersede(ctx context.Context, oldID, newID string) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx,
		`UPDATE decisions SET superseded_by = $1, valid_until = COALESCE(valid_until, NOW())
		 WHERE tenant_id = $2 AND id = $3`, newID, tenantID, oldID)
	return err
}

// CountByCategory returns decision counts per category for the tenant.
func (r *DecisionRepository) CountByCategory(ctx context.Context) (map[string]int, error) {
	tenantID := tenant.FromContext(ctx)
	rows, err := r.pool.Query(ctx,
		`SELECT category, COUNT(*) FROM decisions WHERE tenant_id = $1 GROUP BY category ORDER BY 2 DESC`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var cat string
		var n int
		if err := rows.Scan(&cat, &n); err != nil {
			return nil, err
		}
		out[cat] = n
	}
	return out, rows.Err()
}

func coalesceStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
