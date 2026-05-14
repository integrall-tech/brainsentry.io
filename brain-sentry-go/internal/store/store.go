// Package store defines a backend-agnostic interface for the memory store.
//
// The interface is the *minimum* surface needed for the embedded zero-config
// path (CRUD + text search) so a developer can `brainsentry init --embedded`
// and have a working environment in under 2 seconds with no Postgres,
// no Redis, no FalkorDB. The full production path keeps using
// internal/repository/postgres directly — the goal here is not to refactor
// the whole codebase onto an interface (huge churn for small win) but to
// give the demo / single-developer / OSS-onboarding path a real escape
// hatch from the docker-compose dependency wall.
//
// Two implementations:
//   - PostgresStore (internal/store/postgres_store.go) wraps the existing
//     postgres.MemoryRepository so the embedded surface stays consistent
//     with production semantics where overlap exists.
//   - EmbeddedStore (internal/store/embedded_store.go) backed by SQLite
//     in ~/.brainsentry/brain.db with a tiny BoW relevance scoring for
//     text search. Good enough for development, demos and small operators.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is the canonical "no row" sentinel. Implementations must
// errors.Is against this.
var ErrNotFound = errors.New("memory not found")

// MemoryRecord is the engine-neutral row. We intentionally avoid pulling in
// internal/domain.Memory here so this package has no upward dependency on
// the rest of the code (the postgres/embedded adapters convert at the
// boundary).
type MemoryRecord struct {
	ID         string
	TenantID   string
	Content    string
	Summary    string
	Category   string
	Importance string
	Tags       []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// MemoryStore is the small-surface contract that every backend implements.
// Methods accept context.Context for cancellation and tenant scoping (impls
// read the tenant ID from ctx via pkg/tenant).
type MemoryStore interface {
	// Create inserts a new memory and returns the row with stamped ID +
	// timestamps. ID may be supplied (idempotent upserts) or empty
	// (implementation generates UUID).
	Create(ctx context.Context, m MemoryRecord) (MemoryRecord, error)

	// Get returns one memory by ID, scoped to the ctx tenant. Returns
	// ErrNotFound when missing.
	Get(ctx context.Context, id string) (MemoryRecord, error)

	// List returns memories ordered by CreatedAt DESC, scoped to tenant.
	// limit 0 => no cap (impls may impose a hard ceiling for safety).
	List(ctx context.Context, limit int) ([]MemoryRecord, error)

	// Search runs a text query and returns top-N matches ordered by
	// relevance (impl-defined: postgres uses tsvector, embedded uses BoW).
	Search(ctx context.Context, query string, limit int) ([]MemoryRecord, error)

	// Delete removes a memory by ID. Idempotent — no error for missing IDs.
	Delete(ctx context.Context, id string) error

	// Close releases backend resources (DB pool, file handle, etc.).
	Close() error
}
