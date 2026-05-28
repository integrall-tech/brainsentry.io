package rebuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/integraltech/brainsentry/internal/domain"
)

// --- Dependency interfaces ---
//
// We declare the smallest possible surface each rebuilder needs (instead
// of importing concrete repository / service types) so:
//   - tests can inject fakes without hauling in pgxpool / FalkorDB clients
//   - circular imports stay impossible by construction
//   - swapping a backend (PG → embedded) needs only an interface re-impl

// MemoryLister is the canonical-source side: walks every memory we want
// to re-emit into the derived store. Implementations should iterate by
// page/cursor; the Rebuilder is responsible for chunking.
type MemoryLister interface {
	List(ctx context.Context, page, size int) ([]domain.Memory, int64, error)
}

// GraphSink is the FalkorDB side. SaveToGraph re-inserts a memory node;
// CreateAllRelationships rebuilds tag/manual edges for the tenant set.
// DropGraph fully wipes the graph so the rebuild starts from a known
// state.
type GraphSink interface {
	SaveToGraph(ctx context.Context, m *domain.Memory) error
	CreateAllRelationships(ctx context.Context, tenantID string) error
	DropGraph(ctx context.Context) error
}

// EmbeddingNullifier truncates embeddings on every memory so the next
// search re-embeds lazily. We intentionally do NOT eagerly re-embed in
// this pass — an eager pass burns embedding tokens for memories that
// may never be searched again. Operators who want eager re-embed run
// the embeddings rebuild followed by a touch-all-search batch.
type EmbeddingNullifier interface {
	NullifyAllEmbeddings(ctx context.Context) (int64, error)
}

// CommunityRebuilder owns the Louvain / community-detection step.
type CommunityRebuilder interface {
	DetectAllTenants(ctx context.Context) (int, error)
}

// SummaryWiper truncates the LLM-compressed context_summaries family.
// Re-compression happens lazily on next session load.
type SummaryWiper interface {
	WipeAllContextSummaries(ctx context.Context) (int64, error)
}

// --- Rebuilder factories ---

// GraphRebuilder returns a Rebuilder that drops the FalkorDB graph and
// re-walks memories to re-insert nodes + edges. Pages through memories in
// chunks of 200 to bound memory pressure for large tenants.
func GraphRebuilder(lister MemoryLister, sink GraphSink) Rebuilder {
	return func(ctx context.Context) (int, error) {
		if lister == nil || sink == nil {
			return 0, errors.New("graph rebuilder: lister or sink missing")
		}
		if err := sink.DropGraph(ctx); err != nil {
			return 0, fmt.Errorf("drop graph: %w", err)
		}
		page := 0
		const pageSize = 200
		touched := 0
		seenTenants := map[string]struct{}{}
		// Keep paging until the lister returns an empty batch. Stopping on
		// `len(batch) < pageSize` would short-circuit multi-page rebuilds
		// where the lister returns sub-page batches by design (some impls
		// chunk on tenant boundaries).
		for {
			batch, _, err := lister.List(ctx, page, pageSize)
			if err != nil {
				return touched, fmt.Errorf("list memories page %d: %w", page, err)
			}
			if len(batch) == 0 {
				break
			}
			for i := range batch {
				m := &batch[i]
				if err := sink.SaveToGraph(ctx, m); err != nil {
					return touched, fmt.Errorf("save memory %s: %w", m.ID, err)
				}
				touched++
				seenTenants[m.TenantID] = struct{}{}
			}
			page++
		}
		// Per-tenant edge rebuild — relationships are tenant-scoped in the
		// repository; rebuild edges once we know the full tenant set.
		for t := range seenTenants {
			if err := sink.CreateAllRelationships(ctx, t); err != nil {
				return touched, fmt.Errorf("create relationships for tenant %s: %w", t, err)
			}
		}
		return touched, nil
	}
}

// EmbeddingsRebuilder returns a Rebuilder that nullifies every memory
// embedding. Lazy re-embed avoids wasting tokens; an explicit eager
// rebuild can be a separate target later if usage warrants it.
func EmbeddingsRebuilder(nuller EmbeddingNullifier) Rebuilder {
	return func(ctx context.Context) (int, error) {
		if nuller == nil {
			return 0, errors.New("embeddings rebuilder: nuller missing")
		}
		n, err := nuller.NullifyAllEmbeddings(ctx)
		if err != nil {
			return 0, fmt.Errorf("nullify embeddings: %w", err)
		}
		return int(n), nil
	}
}

// CommunitiesRebuilder returns a Rebuilder that re-detects communities
// for every tenant. Implementations call Louvain (or whichever algorithm
// is current); the rebuild contract just guarantees a fresh result.
func CommunitiesRebuilder(detector CommunityRebuilder) Rebuilder {
	return func(ctx context.Context) (int, error) {
		if detector == nil {
			return 0, errors.New("communities rebuilder: detector missing")
		}
		n, err := detector.DetectAllTenants(ctx)
		if err != nil {
			return 0, fmt.Errorf("detect communities: %w", err)
		}
		return n, nil
	}
}

// CompressRebuilder returns a Rebuilder that truncates LLM-compressed
// context_summaries (and child rows). Next session load re-compresses.
func CompressRebuilder(wiper SummaryWiper) Rebuilder {
	return func(ctx context.Context) (int, error) {
		if wiper == nil {
			return 0, errors.New("compress rebuilder: wiper missing")
		}
		n, err := wiper.WipeAllContextSummaries(ctx)
		if err != nil {
			return 0, fmt.Errorf("wipe context summaries: %w", err)
		}
		return int(n), nil
	}
}
