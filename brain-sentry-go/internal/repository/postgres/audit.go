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

// AuditRepository handles audit log persistence.
type AuditRepository struct {
	pool *pgxpool.Pool
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

const auditColumns = `id, event_type, timestamp, user_id, session_id, user_request,
	decision, reasoning, confidence, input_data, output_data,
	latency_ms, llm_calls, tokens_used, outcome, error_message, user_feedback, tenant_id`

func scanAuditLogs(rows pgx.Rows) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		err := rows.Scan(
			&a.ID, &a.EventType, &a.Timestamp, &a.UserID, &a.SessionID, &a.UserRequest,
			&a.Decision, &a.Reasoning, &a.Confidence, &a.InputData, &a.OutputData,
			&a.LatencyMs, &a.LLMCalls, &a.TokensUsed, &a.Outcome, &a.ErrorMessage, &a.UserFeedback, &a.TenantID,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning audit log: %w", err)
		}
		logs = append(logs, a)
	}
	return logs, nil
}

// Create inserts a new audit log entry with element collections.
func (r *AuditRepository) Create(ctx context.Context, a *domain.AuditLog) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now()
	}
	if a.TenantID == "" {
		a.TenantID = tenant.FromContext(ctx)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(`INSERT INTO audit_logs (%s) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, auditColumns)

	_, err = tx.Exec(ctx, query,
		a.ID, a.EventType, a.Timestamp, a.UserID, a.SessionID, a.UserRequest,
		a.Decision, a.Reasoning, a.Confidence, a.InputData, a.OutputData,
		a.LatencyMs, a.LLMCalls, a.TokensUsed, a.Outcome, a.ErrorMessage, a.UserFeedback, a.TenantID,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}

	// Insert element collections
	if err := r.insertStringCollection(ctx, tx, "audit_memories_accessed", "audit_log_id", "memory_id", a.ID, a.MemoriesAccessed); err != nil {
		return err
	}
	if err := r.insertStringCollection(ctx, tx, "audit_memories_created", "audit_log_id", "memory_id", a.ID, a.MemoriesCreated); err != nil {
		return err
	}
	if err := r.insertStringCollection(ctx, tx, "audit_memories_modified", "audit_log_id", "memory_id", a.ID, a.MemoriesModified); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListByTenant returns audit logs for the current tenant.
func (r *AuditRepository) ListByTenant(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM audit_logs WHERE tenant_id = $1 ORDER BY timestamp DESC LIMIT $2`, auditColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing audit logs: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// FindByEventType returns audit logs filtered by event type.
func (r *AuditRepository) FindByEventType(ctx context.Context, eventType string, limit int) ([]domain.AuditLog, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM audit_logs WHERE tenant_id = $1 AND event_type = $2 ORDER BY timestamp DESC LIMIT $3`, auditColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, eventType, limit)
	if err != nil {
		return nil, fmt.Errorf("finding by event type: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// FindByUserID returns audit logs for a specific user.
func (r *AuditRepository) FindByUserID(ctx context.Context, userID string, limit int) ([]domain.AuditLog, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM audit_logs WHERE tenant_id = $1 AND user_id = $2 ORDER BY timestamp DESC LIMIT $3`, auditColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("finding by user: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// FindBySessionID returns audit logs for a specific session.
func (r *AuditRepository) FindBySessionID(ctx context.Context, sessionID string, limit int) ([]domain.AuditLog, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM audit_logs WHERE tenant_id = $1 AND session_id = $2 ORDER BY timestamp DESC LIMIT $3`, auditColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("finding by session: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// FindByDateRange returns audit logs within a date range.
func (r *AuditRepository) FindByDateRange(ctx context.Context, from, to time.Time, limit int) ([]domain.AuditLog, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM audit_logs WHERE tenant_id = $1 AND timestamp >= $2 AND timestamp <= $3 ORDER BY timestamp DESC LIMIT $4`, auditColumns)

	rows, err := r.pool.Query(ctx, query, tenantID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("finding by date range: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// FindRecent returns the most recent audit logs.
func (r *AuditRepository) FindRecent(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	return r.ListByTenant(ctx, limit)
}

// CountByEventType returns counts grouped by event type.
func (r *AuditRepository) CountByEventType(ctx context.Context) (map[string]int64, error) {
	tenantID := tenant.FromContext(ctx)
	rows, err := r.pool.Query(ctx,
		`SELECT event_type, COUNT(*) FROM audit_logs WHERE tenant_id = $1 GROUP BY event_type`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var et string
		var count int64
		if err := rows.Scan(&et, &count); err != nil {
			return nil, err
		}
		result[et] = count
	}
	return result, nil
}

// CountToday returns the count of audit logs created today.
func (r *AuditRepository) CountToday(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	today := time.Now().Truncate(24 * time.Hour)
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1 AND timestamp >= $2`,
		tenantID, today).Scan(&count)
	return count, err
}

// CountByEventTypeValue returns the count for a specific event type.
func (r *AuditRepository) CountByEventTypeValue(ctx context.Context, eventType string) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1 AND event_type = $2`,
		tenantID, eventType).Scan(&count)
	return count, err
}

// AverageLatency returns the average latency for the tenant.
func (r *AuditRepository) AverageLatency(ctx context.Context) (float64, error) {
	tenantID := tenant.FromContext(ctx)
	var avg *float64
	err := r.pool.QueryRow(ctx,
		`SELECT AVG(latency_ms) FROM audit_logs WHERE tenant_id = $1 AND latency_ms IS NOT NULL`,
		tenantID).Scan(&avg)
	if err != nil || avg == nil {
		return 0, err
	}
	return *avg, nil
}

// FindByMemoryID returns audit logs that reference a specific memory (accessed, created, or modified).
func (r *AuditRepository) FindByMemoryID(ctx context.Context, memoryID string, limit int) ([]domain.AuditLog, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT DISTINCT %s FROM audit_logs a
		WHERE a.tenant_id = $1 AND (
			EXISTS (SELECT 1 FROM audit_memories_accessed ma WHERE ma.audit_log_id = a.id AND ma.memory_id = $2)
			OR EXISTS (SELECT 1 FROM audit_memories_created mc WHERE mc.audit_log_id = a.id AND mc.memory_id = $2)
			OR EXISTS (SELECT 1 FROM audit_memories_modified mm WHERE mm.audit_log_id = a.id AND mm.memory_id = $2)
		)
		ORDER BY a.timestamp DESC LIMIT $3`, "a."+strings.ReplaceAll(auditColumns, ", ", ", a."))

	rows, err := r.pool.Query(ctx, query, tenantID, memoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("finding audit by memory: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

func (r *AuditRepository) insertStringCollection(ctx context.Context, tx pgx.Tx, table, parentCol, valueCol, parentID string, values []string) error {
	if len(values) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES ", table, parentCol, valueCol))
	args := make([]any, 0, len(values)*2)
	for i, v := range values {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		args = append(args, parentID, v)
	}
	sb.WriteString(" ON CONFLICT DO NOTHING")
	_, err := tx.Exec(ctx, sb.String(), args...)
	return err
}

// ToJSON converts a map to JSON for storage.
func ToJSON(data map[string]any) json.RawMessage {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	return b
}
