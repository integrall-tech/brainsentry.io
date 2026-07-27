package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
)

// APIKeyRepository persists service API keys.
type APIKeyRepository struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepository creates a new APIKeyRepository.
func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

const apiKeyColumns = `id, tenant_id, name, key_prefix, key_hash, created_at,
	created_by, last_used_at, revoked_at, expires_at`

func scanAPIKey(row pgx.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	var createdBy *string
	if err := row.Scan(
		&k.ID, &k.TenantID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.CreatedAt,
		&createdBy, &k.LastUsedAt, &k.RevokedAt, &k.ExpiresAt,
	); err != nil {
		return nil, err
	}
	if createdBy != nil {
		k.CreatedBy = *createdBy
	}
	return &k, nil
}

// Create stores a new key. The caller is responsible for having hashed the
// secret — this layer never sees the plaintext.
func (r *APIKeyRepository) Create(ctx context.Context, k *domain.APIKey) error {
	if k.ID == "" {
		k.ID = uuid.NewString()
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}

	var createdBy *string
	if k.CreatedBy != "" {
		createdBy = &k.CreatedBy
	}

	query := fmt.Sprintf(`INSERT INTO api_keys (%s)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, apiKeyColumns)

	_, err := r.pool.Exec(ctx, query,
		k.ID, k.TenantID, k.Name, k.KeyPrefix, k.KeyHash, k.CreatedAt,
		createdBy, k.LastUsedAt, k.RevokedAt, k.ExpiresAt)
	if err != nil {
		return fmt.Errorf("inserting api key: %w", err)
	}
	return nil
}

// FindCandidatesByPrefix returns live keys sharing a lookup prefix.
//
// Returns a slice, not a single row, on purpose: the prefix is a lookup
// handle, not an identity. Two keys could collide on it, and letting the
// caller compare every candidate in constant time keeps the authorisation
// decision in one place — the hash — instead of splitting it between the
// index and the comparison.
func (r *APIKeyRepository) FindCandidatesByPrefix(ctx context.Context, prefix string) ([]*domain.APIKey, error) {
	query := fmt.Sprintf(`SELECT %s FROM api_keys
		WHERE key_prefix = $1 AND revoked_at IS NULL`, apiKeyColumns)

	rows, err := r.pool.Query(ctx, query, prefix)
	if err != nil {
		return nil, fmt.Errorf("finding api keys by prefix: %w", err)
	}
	defer rows.Close()

	var out []*domain.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListByTenant returns every key of a tenant, newest first, including revoked
// ones — an operator needs to see what was revoked and when.
func (r *APIKeyRepository) ListByTenant(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	query := fmt.Sprintf(`SELECT %s FROM api_keys
		WHERE tenant_id = $1 ORDER BY created_at DESC`, apiKeyColumns)

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	defer rows.Close()

	var out []*domain.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// FindByID returns a single key regardless of tenant. Callers in the admin
// surface MUST check the tenant themselves before acting on it.
func (r *APIKeyRepository) FindByID(ctx context.Context, id string) (*domain.APIKey, error) {
	query := fmt.Sprintf(`SELECT %s FROM api_keys WHERE id = $1`, apiKeyColumns)
	k, err := scanAPIKey(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("api key not found: %s", id)
		}
		return nil, fmt.Errorf("finding api key: %w", err)
	}
	return k, nil
}

// Revoke marks a key as revoked. Idempotent: revoking an already-revoked key
// keeps the original timestamp, so the audit trail is not rewritten by a
// second call.
func (r *APIKeyRepository) Revoke(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = COALESCE(revoked_at, NOW()) WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}
	return nil
}

// lastUsedThrottle is how stale last_used_at is allowed to get. Without it,
// every authenticated request would turn into a write — on the recall path
// that is a row lock and a WAL record per read.
const lastUsedThrottle = 5 * time.Minute

// TouchLastUsed records that a key was used, at most once per throttle
// window.
//
// Best-effort by contract: the caller ignores the error. Failing a request
// because bookkeeping failed would trade a working integration for a
// statistic.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW()
		 WHERE id = $1
		   AND (last_used_at IS NULL OR last_used_at < NOW() - $2::interval)`,
		id, lastUsedThrottle.String())
	return err
}
