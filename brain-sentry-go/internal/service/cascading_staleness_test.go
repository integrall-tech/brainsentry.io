package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

// stalenessMockRepo implements stalenessMemoryRepository and
// stalenessRelationshipRepository over in-memory maps.
type stalenessMockRepo struct {
	memories map[string]*domain.Memory
	rels     []domain.MemoryRelationship
	updates  []string // IDs passed to Update, in order
	findErr  error
}

func (m *stalenessMockRepo) FindByID(_ context.Context, id string) (*domain.Memory, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	mem, ok := m.memories[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return mem, nil
}

func (m *stalenessMockRepo) Update(_ context.Context, mem *domain.Memory) error {
	m.updates = append(m.updates, mem.ID)
	m.memories[mem.ID] = mem
	return nil
}

func (m *stalenessMockRepo) FindByFromMemoryID(_ context.Context, memoryID string) ([]domain.MemoryRelationship, error) {
	var out []domain.MemoryRelationship
	for _, r := range m.rels {
		if r.FromMemoryID == memoryID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *stalenessMockRepo) FindByToMemoryID(_ context.Context, memoryID string) ([]domain.MemoryRelationship, error) {
	var out []domain.MemoryRelationship
	for _, r := range m.rels {
		if r.ToMemoryID == memoryID {
			out = append(out, r)
		}
	}
	return out, nil
}

func newStalenessService(repo *stalenessMockRepo) *CascadingStalenessService {
	return &CascadingStalenessService{
		memoryRepo:       repo,
		relationshipRepo: repo,
		maxDepth:         3,
	}
}

func mem(id string) *domain.Memory {
	return &domain.Memory{ID: id}
}

func rel(from, to string) domain.MemoryRelationship {
	return domain.MemoryRelationship{ID: from + "->" + to, FromMemoryID: from, ToMemoryID: to}
}

func TestStaleness_DirectNeighborsMarkedStale(t *testing.T) {
	// old -> a, b <- old (both directions count as neighbors)
	repo := &stalenessMockRepo{
		memories: map[string]*domain.Memory{
			"old": mem("old"), "new": mem("new"), "a": mem("a"), "b": mem("b"),
		},
		rels: []domain.MemoryRelationship{rel("old", "a"), rel("b", "old")},
	}
	svc := newStalenessService(repo)

	result, err := svc.PropagateFromSupersession(context.Background(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.MarkedStale) != 2 {
		t.Fatalf("expected 2 direct neighbors marked stale, got %v", result.MarkedStale)
	}
	for _, id := range []string{"a", "b"} {
		var meta map[string]any
		if err := json.Unmarshal(repo.memories[id].Metadata, &meta); err != nil {
			t.Fatalf("metadata of %s not valid JSON: %v", id, err)
		}
		if meta["staleness_source"] != "old" {
			t.Errorf("%s: expected staleness_source=old, got %v", id, meta["staleness_source"])
		}
		if meta["needs_review"] != true {
			t.Errorf("%s: expected needs_review=true", id)
		}
	}
}

func TestStaleness_SecondHopMarkedForReviewNotStale(t *testing.T) {
	// old -> a -> c : "a" fica stale (hop 1), "c" fica needs_review (hop 2).
	repo := &stalenessMockRepo{
		memories: map[string]*domain.Memory{
			"old": mem("old"), "new": mem("new"), "a": mem("a"), "c": mem("c"),
		},
		rels: []domain.MemoryRelationship{rel("old", "a"), rel("a", "c")},
	}
	svc := newStalenessService(repo)

	result, err := svc.PropagateFromSupersession(context.Background(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.MarkedStale) != 1 || result.MarkedStale[0] != "a" {
		t.Errorf("expected only 'a' stale, got %v", result.MarkedStale)
	}
	if len(result.MarkedForReview) != 1 || result.MarkedForReview[0] != "c" {
		t.Errorf("expected only 'c' for review, got %v", result.MarkedForReview)
	}

	var meta map[string]any
	_ = json.Unmarshal(repo.memories["c"].Metadata, &meta)
	if meta["needs_review_because"] != "old" {
		t.Errorf("c: expected needs_review_because=old, got %v", meta["needs_review_because"])
	}
	if _, hasStale := meta["staleness_source"]; hasStale {
		t.Error("c: second hop must not carry staleness_source")
	}
}

func TestStaleness_CycleDoesNotLoop(t *testing.T) {
	// old -> a -> b -> old (ciclo). Visited deve impedir reprocessamento.
	repo := &stalenessMockRepo{
		memories: map[string]*domain.Memory{
			"old": mem("old"), "new": mem("new"), "a": mem("a"), "b": mem("b"),
		},
		rels: []domain.MemoryRelationship{rel("old", "a"), rel("a", "b"), rel("b", "old")},
	}
	svc := newStalenessService(repo)

	result, err := svc.PropagateFromSupersession(context.Background(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cada nó marcado exatamente uma vez; "old"/"new" nunca marcados.
	if len(result.MarkedStale)+len(result.MarkedForReview) != 2 {
		t.Errorf("expected exactly 2 marks total, got stale=%v review=%v",
			result.MarkedStale, result.MarkedForReview)
	}
	for _, id := range result.MarkedStale {
		if id == "old" || id == "new" {
			t.Errorf("source/new must never be marked, got %s", id)
		}
	}
}

func TestStaleness_PropagationStopsAfterReviewHop(t *testing.T) {
	// Cadeia old -> n1 -> n2 -> n3 -> n4. Só nós marcados como stale (hop 1)
	// alimentam o próximo nível do BFS; nós em review (hop 2+) não fan-out.
	// Logo a propagação efetiva é: n1 stale, n2 review, n3/n4 intocados —
	// mesmo com maxDepth=3. Se isso mudar de propósito, atualize este teste.
	memories := map[string]*domain.Memory{"old": mem("old"), "new": mem("new")}
	var rels []domain.MemoryRelationship
	prev := "old"
	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("n%d", i)
		memories[id] = mem(id)
		rels = append(rels, rel(prev, id))
		prev = id
	}
	repo := &stalenessMockRepo{memories: memories, rels: rels}
	svc := newStalenessService(repo)

	result, err := svc.PropagateFromSupersession(context.Background(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.MarkedStale) != 1 || result.MarkedStale[0] != "n1" {
		t.Errorf("expected only n1 stale, got %v", result.MarkedStale)
	}
	if len(result.MarkedForReview) != 1 || result.MarkedForReview[0] != "n2" {
		t.Errorf("expected only n2 for review, got %v", result.MarkedForReview)
	}
	for _, id := range []string{"n3", "n4"} {
		if len(repo.memories[id].Metadata) != 0 {
			t.Errorf("%s must remain untouched", id)
		}
	}
}

func TestStaleness_SkipsDeletedAndAlreadySuperseded(t *testing.T) {
	now := time.Now()
	deleted := mem("deleted")
	deleted.DeletedAt = &now
	superseded := mem("superseded")
	superseded.SupersededBy = "x"

	repo := &stalenessMockRepo{
		memories: map[string]*domain.Memory{
			"old": mem("old"), "new": mem("new"),
			"deleted": deleted, "superseded": superseded, "ok": mem("ok"),
		},
		rels: []domain.MemoryRelationship{
			rel("old", "deleted"), rel("old", "superseded"), rel("old", "ok"),
		},
	}
	svc := newStalenessService(repo)

	result, err := svc.PropagateFromSupersession(context.Background(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.MarkedStale) != 1 || result.MarkedStale[0] != "ok" {
		t.Errorf("expected only 'ok' marked (deleted/superseded skipped), got %v", result.MarkedStale)
	}
}

func TestStaleness_PreservesExistingMetadata(t *testing.T) {
	a := mem("a")
	a.Metadata = json.RawMessage(`{"keep":"me"}`)
	repo := &stalenessMockRepo{
		memories: map[string]*domain.Memory{"old": mem("old"), "new": mem("new"), "a": a},
		rels:     []domain.MemoryRelationship{rel("old", "a")},
	}
	svc := newStalenessService(repo)

	if _, err := svc.PropagateFromSupersession(context.Background(), "old", "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal(repo.memories["a"].Metadata, &meta); err != nil {
		t.Fatalf("invalid metadata JSON: %v", err)
	}
	if meta["keep"] != "me" {
		t.Errorf("existing metadata key lost: %v", meta)
	}
	if meta["staleness_source"] != "old" {
		t.Errorf("staleness_source missing: %v", meta)
	}
}

func TestStaleness_NoRelationshipsNoMarks(t *testing.T) {
	repo := &stalenessMockRepo{
		memories: map[string]*domain.Memory{"old": mem("old"), "new": mem("new")},
	}
	svc := newStalenessService(repo)

	result, err := svc.PropagateFromSupersession(context.Background(), "old", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MarkedStale) != 0 || len(result.MarkedForReview) != 0 || len(repo.updates) != 0 {
		t.Errorf("isolated memory must produce zero marks, got %+v updates=%v", result, repo.updates)
	}
}
