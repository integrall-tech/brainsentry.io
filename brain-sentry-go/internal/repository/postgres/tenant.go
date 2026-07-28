package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
)

// TenantRepository handles tenant persistence in PostgreSQL.
type TenantRepository struct {
	pool *pgxpool.Pool
}

// NewTenantRepository creates a new TenantRepository.
func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{pool: pool}
}

// FindByID finds a tenant by ID.
func (r *TenantRepository) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	query := `
		SELECT id, name, slug, description, active, max_memories, max_users, settings, created_at, updated_at
		FROM tenants WHERE id = $1`

	var t domain.Tenant
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Description, &t.Active,
		&t.MaxMemories, &t.MaxUsers, &t.Settings,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("finding tenant: %w", err)
	}
	return &t, nil
}

// FindBySlug finds a tenant by slug.
func (r *TenantRepository) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	query := `
		SELECT id, name, slug, description, active, max_memories, max_users, settings, created_at, updated_at
		FROM tenants WHERE slug = $1`

	var t domain.Tenant
	err := r.pool.QueryRow(ctx, query, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Description, &t.Active,
		&t.MaxMemories, &t.MaxUsers, &t.Settings,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("finding tenant by slug: %w", err)
	}
	return &t, nil
}

// List returns all tenants.
func (r *TenantRepository) List(ctx context.Context) ([]domain.Tenant, error) {
	query := `
		SELECT id, name, slug, description, active, max_memories, max_users, settings, created_at, updated_at
		FROM tenants ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Slug, &t.Description, &t.Active,
			&t.MaxMemories, &t.MaxUsers, &t.Settings,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

// Create inserts a new tenant.
func (r *TenantRepository) Create(ctx context.Context, t *domain.Tenant) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	if t.Settings == nil {
		t.Settings = json.RawMessage(`{}`)
	}

	query := `
		INSERT INTO tenants (id, name, slug, description, active, max_memories, max_users, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		t.ID, t.Name, t.Slug, t.Description, t.Active,
		t.MaxMemories, t.MaxUsers, t.Settings,
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating tenant: %w", err)
	}
	return nil
}

// Update updates a tenant.
func (r *TenantRepository) Update(ctx context.Context, t *domain.Tenant) error {
	t.UpdatedAt = time.Now()
	query := `
		UPDATE tenants SET name=$1, slug=$2, description=$3, active=$4,
		max_memories=$5, max_users=$6, settings=$7, updated_at=$8
		WHERE id=$9`

	_, err := r.pool.Exec(ctx, query,
		t.Name, t.Slug, t.Description, t.Active,
		t.MaxMemories, t.MaxUsers, t.Settings, t.UpdatedAt,
		t.ID,
	)
	if err != nil {
		return fmt.Errorf("updating tenant: %w", err)
	}
	return nil
}

// Delete deletes a tenant.
func (r *TenantRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting tenant: %w", err)
	}
	return nil
}

// TenantStats is the per-tenant row the admin UI lists.
type TenantStats struct {
	TenantID          string `json:"tenantId"`
	MemoryCount       int64  `json:"memoryCount"`
	UserCount         int64  `json:"userCount"`
	RelationshipCount int64  `json:"relationshipCount"`
}

// Stats returns counts per tenant.
//
// Counts come from a LEFT JOIN on the tenant list so a tenant with no data
// still appears with zeros: the admin UI keys its table by tenantId, and a
// missing row would render blank cells that read as "loading" rather than
// "empty".
func (r *TenantRepository) Stats(ctx context.Context) ([]TenantStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id,
		       (SELECT count(*) FROM memories m
		         WHERE m.tenant_id = t.id AND m.deleted_at IS NULL),
		       (SELECT count(*) FROM users u WHERE u.tenant_id = t.id),
		       (SELECT count(*) FROM memory_relationships mr WHERE mr.tenant_id = t.id)
		FROM tenants t
		ORDER BY t.created_at`)
	if err != nil {
		return nil, fmt.Errorf("computing tenant stats: %w", err)
	}
	defer rows.Close()

	out := []TenantStats{}
	for rows.Next() {
		var s TenantStats
		if err := rows.Scan(&s.TenantID, &s.MemoryCount, &s.UserCount, &s.RelationshipCount); err != nil {
			return nil, fmt.Errorf("scanning tenant stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
