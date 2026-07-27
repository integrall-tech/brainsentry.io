package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
)

// ReceiptRepository persists erasure/retention receipts.
type ReceiptRepository struct {
	pool *pgxpool.Pool
}

// NewReceiptRepository creates a new ReceiptRepository.
func NewReceiptRepository(pool *pgxpool.Pool) *ReceiptRepository {
	return &ReceiptRepository{pool: pool}
}

// Create stores a receipt.
func (r *ReceiptRepository) Create(ctx context.Context, rec *domain.ErasureReceipt) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO erasure_receipts
		 (id, tenant_id, kind, scope, counts, reason, requested_by, executed, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rec.ID, rec.TenantID, rec.Kind, rec.Scope, rec.Counts,
		nullIfEmpty(rec.Reason), nullIfEmpty(rec.RequestedBy), rec.Executed, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting erasure receipt: %w", err)
	}
	return nil
}

// ListByTenant returns a tenant's receipts, newest first — the surface an
// operator uses to answer "prove this was removed".
func (r *ReceiptRepository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]*domain.ErasureReceipt, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, kind, scope, counts, COALESCE(reason,''), COALESCE(requested_by,''), executed, created_at
		 FROM erasure_receipts WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2`,
		tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing erasure receipts: %w", err)
	}
	defer rows.Close()

	var out []*domain.ErasureReceipt
	for rows.Next() {
		var rec domain.ErasureReceipt
		if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.Kind, &rec.Scope, &rec.Counts,
			&rec.Reason, &rec.RequestedBy, &rec.Executed, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &rec)
	}
	return out, rows.Err()
}
