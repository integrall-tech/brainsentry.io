package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// ContextSummaryRepository handles context summary persistence.
type ContextSummaryRepository struct {
	pool *pgxpool.Pool
}

// NewContextSummaryRepository creates a new ContextSummaryRepository.
func NewContextSummaryRepository(pool *pgxpool.Pool) *ContextSummaryRepository {
	return &ContextSummaryRepository{pool: pool}
}

// Create saves a context summary.
func (r *ContextSummaryRepository) Create(ctx context.Context, cs *domain.ContextSummary) error {
	if cs.ID == "" {
		cs.ID = uuid.New().String()
	}
	if cs.TenantID == "" {
		cs.TenantID = tenant.FromContext(ctx)
	}
	cs.CreatedAt = time.Now()

	query := `INSERT INTO context_summaries (id, tenant_id, session_id,
		original_token_count, compressed_token_count, compression_ratio,
		summary, recent_window_size, created_at, model_used, compression_method)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.pool.Exec(ctx, query,
		cs.ID, cs.TenantID, cs.SessionID,
		cs.OriginalTokenCount, cs.CompressedTokenCount, cs.CompressionRatio,
		cs.Summary, cs.RecentWindowSize, cs.CreatedAt, cs.ModelUsed, cs.CompressionMethod,
	)
	if err != nil {
		return fmt.Errorf("creating context summary: %w", err)
	}

	// Save collection items. A failure here leaves the summary partially
	// persisted, so surface it rather than silently dropping rows.
	for _, g := range cs.Goals {
		if _, err := r.pool.Exec(ctx, `INSERT INTO context_summary_goals (context_summary_id, goal) VALUES ($1, $2)`, cs.ID, g); err != nil {
			return fmt.Errorf("saving context summary goal: %w", err)
		}
	}
	for _, d := range cs.Decisions {
		if _, err := r.pool.Exec(ctx, `INSERT INTO context_summary_decisions (context_summary_id, decision) VALUES ($1, $2)`, cs.ID, d); err != nil {
			return fmt.Errorf("saving context summary decision: %w", err)
		}
	}
	for _, e := range cs.Errors {
		if _, err := r.pool.Exec(ctx, `INSERT INTO context_summary_errors (context_summary_id, error) VALUES ($1, $2)`, cs.ID, e); err != nil {
			return fmt.Errorf("saving context summary error: %w", err)
		}
	}
	for _, t := range cs.Todos {
		if _, err := r.pool.Exec(ctx, `INSERT INTO context_summary_todos (context_summary_id, todo) VALUES ($1, $2)`, cs.ID, t); err != nil {
			return fmt.Errorf("saving context summary todo: %w", err)
		}
	}

	return nil
}

// FindBySession returns context summaries for a session.
func (r *ContextSummaryRepository) FindBySession(ctx context.Context, sessionID string) ([]domain.ContextSummary, error) {
	tenantID := tenant.FromContext(ctx)

	query := `SELECT id, tenant_id, session_id, original_token_count, compressed_token_count,
		compression_ratio, summary, recent_window_size, created_at, model_used, compression_method
		FROM context_summaries WHERE tenant_id = $1 AND session_id = $2
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("finding context summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.ContextSummary
	for rows.Next() {
		var cs domain.ContextSummary
		if err := rows.Scan(
			&cs.ID, &cs.TenantID, &cs.SessionID,
			&cs.OriginalTokenCount, &cs.CompressedTokenCount, &cs.CompressionRatio,
			&cs.Summary, &cs.RecentWindowSize, &cs.CreatedAt, &cs.ModelUsed, &cs.CompressionMethod,
		); err != nil {
			return nil, fmt.Errorf("scanning context summary: %w", err)
		}

		// Load collections
		cs.Goals = r.loadCollection(ctx, "context_summary_goals", "goal", cs.ID)
		cs.Decisions = r.loadCollection(ctx, "context_summary_decisions", "decision", cs.ID)
		cs.Errors = r.loadCollection(ctx, "context_summary_errors", "error", cs.ID)
		cs.Todos = r.loadCollection(ctx, "context_summary_todos", "todo", cs.ID)

		summaries = append(summaries, cs)
	}
	return summaries, nil
}

// GetLatestBySession returns the most recent context summary for a session.
func (r *ContextSummaryRepository) GetLatestBySession(ctx context.Context, sessionID string) (*domain.ContextSummary, error) {
	summaries, err := r.FindBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if len(summaries) == 0 {
		return nil, fmt.Errorf("no context summary found for session %s", sessionID)
	}
	return &summaries[0], nil
}

func (r *ContextSummaryRepository) loadCollection(ctx context.Context, table, column, summaryID string) []string {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE context_summary_id = $1`, column, table)
	rows, err := r.pool.Query(ctx, query, summaryID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err == nil {
			items = append(items, item)
		}
	}
	return items
}
