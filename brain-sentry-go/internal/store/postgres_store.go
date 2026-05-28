package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// PostgresStore adapts the existing *postgres.MemoryRepository to the
// MemoryStore interface. The intent is to prove the abstraction works in
// production — handlers wired to MemoryStore can be swapped between this
// (Postgres + pgvector) and EmbeddedStore (JSON file) by config alone.
//
// We do NOT touch the existing MemoryRepository surface. Several services
// hold direct references and use repository methods that the small
// MemoryStore interface intentionally does not expose (versioning,
// embeddings, cross-tenant analytics). This adapter is additive.
type PostgresStore struct {
	repo *postgres.MemoryRepository
}

// NewPostgresStore wraps an existing repository.
func NewPostgresStore(r *postgres.MemoryRepository) *PostgresStore {
	return &PostgresStore{repo: r}
}

// Create inserts a memory. Stamps a UUID when ID is empty and pulls
// TenantID from ctx when not set.
func (s *PostgresStore) Create(ctx context.Context, m MemoryRecord) (MemoryRecord, error) {
	if s.repo == nil {
		return MemoryRecord{}, errors.New("postgres store: nil repo")
	}
	dm := toDomain(m)
	if dm.ID == "" {
		dm.ID = uuid.NewString()
	}
	if dm.TenantID == "" {
		dm.TenantID = tenant.FromContext(ctx)
	}
	if err := s.repo.Create(ctx, dm); err != nil {
		return MemoryRecord{}, err
	}
	got, err := s.repo.FindByID(ctx, dm.ID)
	if err != nil {
		// Fall back to the stamped local copy — Create succeeded so the
		// caller still gets a usable record. Rare race; not worth panicking.
		return fromDomain(dm), nil
	}
	return fromDomain(got), nil
}

// Get returns a single memory by ID with tenant scoping enforced inside
// the repository's queries.
func (s *PostgresStore) Get(ctx context.Context, id string) (MemoryRecord, error) {
	if s.repo == nil {
		return MemoryRecord{}, errors.New("postgres store: nil repo")
	}
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		// FindByID wraps pgx.ErrNoRows for missing rows; surface that as
		// the store-level ErrNotFound so handler can map it to 404 instead
		// of 500. Without this check a GET on a freshly-deleted memory
		// 500s, which broke /v1/store/memories' "delete then GET → 404"
		// contract.
		if errors.Is(err, pgx.ErrNoRows) {
			return MemoryRecord{}, ErrNotFound
		}
		return MemoryRecord{}, err
	}
	if m == nil {
		return MemoryRecord{}, ErrNotFound
	}
	return fromDomain(m), nil
}

// List returns memories ordered newest-first. limit 0 falls back to the
// repository's default page size.
func (s *PostgresStore) List(ctx context.Context, limit int) ([]MemoryRecord, error) {
	if s.repo == nil {
		return nil, errors.New("postgres store: nil repo")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, _, err := s.repo.List(ctx, 0, limit)
	if err != nil {
		return nil, err
	}
	out := make([]MemoryRecord, len(rows))
	for i := range rows {
		out[i] = fromDomain(&rows[i])
	}
	return out, nil
}

// Search delegates to the repository's full-text path. Vector search is
// available via FalkorDB — this small surface is for the canonical text
// query handlers should not hard-code the LLM-routed retrieval pipeline.
func (s *PostgresStore) Search(ctx context.Context, query string, limit int) ([]MemoryRecord, error) {
	if s.repo == nil {
		return nil, errors.New("postgres store: nil repo")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.repo.FullTextSearch(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]MemoryRecord, len(rows))
	for i := range rows {
		out[i] = fromDomain(&rows[i])
	}
	return out, nil
}

// Delete removes a memory by ID. The underlying repo treats missing rows
// as success (idempotent), matching the embedded store's behavior.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("postgres store: nil repo")
	}
	return s.repo.Delete(ctx, id)
}

// Close is a no-op — the underlying pgxpool is owned by the server entry
// point, not by this adapter.
func (s *PostgresStore) Close() error { return nil }

// --- Conversions ---

func toDomain(m MemoryRecord) *domain.Memory {
	return &domain.Memory{
		ID:         m.ID,
		Content:    m.Content,
		Summary:    m.Summary,
		Category:   domain.MemoryCategory(m.Category),
		Importance: domain.ImportanceLevel(m.Importance),
		Tags:       m.Tags,
		TenantID:   m.TenantID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

func fromDomain(m *domain.Memory) MemoryRecord {
	if m == nil {
		return MemoryRecord{}
	}
	return MemoryRecord{
		ID:         m.ID,
		TenantID:   m.TenantID,
		Content:    m.Content,
		Summary:    m.Summary,
		Category:   string(m.Category),
		Importance: string(m.Importance),
		Tags:       m.Tags,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}
