package service

import (
	"context"
	"errors"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// mockDecisionRepo is an in-memory decisionRepository for tests.
type mockDecisionRepo struct {
	decisions  map[string]*domain.Decision
	children   map[string][]*domain.Decision
	precedents []*domain.DecisionPrecedent
	countByCat map[string]int

	created    []*domain.Decision
	superseded [][2]string

	createErr     error
	findErr       error
	precedentsErr error
}

func newMockDecisionRepo() *mockDecisionRepo {
	return &mockDecisionRepo{
		decisions: map[string]*domain.Decision{},
		children:  map[string][]*domain.Decision{},
	}
}

func (m *mockDecisionRepo) Create(ctx context.Context, d *domain.Decision) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, d)
	m.decisions[d.ID] = d
	return nil
}

func (m *mockDecisionRepo) FindByID(ctx context.Context, id string) (*domain.Decision, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	d, ok := m.decisions[id]
	if !ok {
		return nil, errors.New("decision not found")
	}
	return d, nil
}

func (m *mockDecisionRepo) List(ctx context.Context, f postgres.DecisionFilter) ([]*domain.Decision, error) {
	out := make([]*domain.Decision, 0, len(m.decisions))
	for _, d := range m.decisions {
		out = append(out, d)
	}
	return out, nil
}

func (m *mockDecisionRepo) Children(ctx context.Context, id string) ([]*domain.Decision, error) {
	return m.children[id], nil
}

func (m *mockDecisionRepo) FindPrecedents(ctx context.Context, category string, embedding []float32, limit int) ([]*domain.DecisionPrecedent, error) {
	if m.precedentsErr != nil {
		return nil, m.precedentsErr
	}
	if limit < len(m.precedents) {
		return m.precedents[:limit], nil
	}
	return m.precedents, nil
}

func (m *mockDecisionRepo) Supersede(ctx context.Context, oldID, newID string) error {
	m.superseded = append(m.superseded, [2]string{oldID, newID})
	return nil
}

func (m *mockDecisionRepo) CountByCategory(ctx context.Context) (map[string]int, error) {
	if m.countByCat == nil {
		return map[string]int{}, nil
	}
	return m.countByCat, nil
}

func newTestDecisionService(repo decisionRepository) *DecisionService {
	return &DecisionService{
		repo:    repo,
		tracker: NewProvenanceTracker("decision", nil),
	}
}

func TestDecisionRecord_HappyPath(t *testing.T) {
	repo := newMockDecisionRepo()
	svc := newTestDecisionService(repo)

	d, err := svc.Record(context.Background(), RecordDecisionRequest{
		Category:  "deploy",
		Scenario:  "rollout to prod",
		Reasoning: "canary was green",
		Outcome:   domain.DecisionApproved,
		Confidence: 0.9,
		MemoryIDs: []string{"m1"},
		Metadata:  map[string]any{"region": "us-east-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.ID == "" {
		t.Error("expected generated ID")
	}
	if d.Outcome != domain.DecisionApproved {
		t.Errorf("unexpected outcome: %s", d.Outcome)
	}
	if d.CreatedAt.IsZero() || d.RecordedAt.IsZero() {
		t.Error("expected timestamps to be set")
	}
	if len(d.Metadata) == 0 {
		t.Error("expected metadata to be marshalled")
	}
	if len(repo.created) != 1 || repo.created[0].ID != d.ID {
		t.Errorf("expected decision persisted once, got %d", len(repo.created))
	}
}

func TestDecisionRecord_DefaultsOutcomeToPending(t *testing.T) {
	svc := newTestDecisionService(newMockDecisionRepo())

	d, err := svc.Record(context.Background(), RecordDecisionRequest{
		Category: "deploy",
		Scenario: "rollback",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Outcome != domain.DecisionPending {
		t.Errorf("expected pending default, got %s", d.Outcome)
	}
}

func TestDecisionRecord_MissingRequiredFields(t *testing.T) {
	repo := newMockDecisionRepo()
	svc := newTestDecisionService(repo)

	cases := []RecordDecisionRequest{
		{},                       // both missing
		{Category: "deploy"},     // scenario missing
		{Scenario: "rollout"},    // category missing
	}
	for i, req := range cases {
		if _, err := svc.Record(context.Background(), req); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
	if len(repo.created) != 0 {
		t.Errorf("nothing should be persisted on validation error, got %d", len(repo.created))
	}
}

func TestDecisionRecord_RepoError(t *testing.T) {
	repo := newMockDecisionRepo()
	repo.createErr = errors.New("db down")
	svc := newTestDecisionService(repo)

	if _, err := svc.Record(context.Background(), RecordDecisionRequest{Category: "c", Scenario: "s"}); err == nil {
		t.Fatal("expected repo error to propagate")
	}
}

func TestDecisionRecord_PolicyViolationsAttached(t *testing.T) {
	policyRepo := newMockPolicyRepo()
	policyRepo.byCategory["deploy"] = []*domain.Policy{{
		ID:       "pol-1",
		Name:     "min confidence",
		Category: "deploy",
		Severity: domain.PolicyError,
		RuleType: domain.PolicyMinConfidence,
		RuleConfig: mustJSON(t, map[string]any{"threshold": 0.8}),
	}}
	engine := &PolicyEngine{repo: policyRepo}

	svc := newTestDecisionService(newMockDecisionRepo()).WithPolicyEngine(engine)

	d, err := svc.Record(context.Background(), RecordDecisionRequest{
		Category:   "deploy",
		Scenario:   "yolo push",
		Confidence: 0.2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.PolicyViolations) != 1 || d.PolicyViolations[0] != "pol-1" {
		t.Errorf("expected violation pol-1 attached, got %#v", d.PolicyViolations)
	}
}

func TestFindPrecedentsForDecision_DropsSelfAndHonoursLimit(t *testing.T) {
	repo := newMockDecisionRepo()
	target := &domain.Decision{ID: "d1", Category: "deploy", Embedding: []float32{0.1, 0.2}}
	repo.decisions["d1"] = target
	repo.precedents = []*domain.DecisionPrecedent{
		{Decision: target, Similarity: 1.0}, // self — must be dropped
		{Decision: &domain.Decision{ID: "d2", Category: "deploy"}, Similarity: 0.9},
		{Decision: &domain.Decision{ID: "d3", Category: "deploy"}, Similarity: 0.8},
	}
	svc := newTestDecisionService(repo)

	got, err := svc.FindPrecedentsForDecision(context.Background(), "d1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Decision.ID != "d2" {
		t.Errorf("expected only d2, got %#v", got)
	}
}

func TestFindPrecedentsForDecision_NotFound(t *testing.T) {
	svc := newTestDecisionService(newMockDecisionRepo())
	if _, err := svc.FindPrecedentsForDecision(context.Background(), "missing", 5); err == nil {
		t.Fatal("expected error for unknown decision")
	}
}

func TestCausalChain_AncestorsAndDescendants(t *testing.T) {
	repo := newMockDecisionRepo()
	root := &domain.Decision{ID: "root"}
	mid := &domain.Decision{ID: "mid", ParentDecisionID: "root"}
	target := &domain.Decision{ID: "target", ParentDecisionID: "mid"}
	childA := &domain.Decision{ID: "childA", ParentDecisionID: "target"}
	grandchild := &domain.Decision{ID: "grandchild", ParentDecisionID: "childA"}
	for _, d := range []*domain.Decision{root, mid, target, childA, grandchild} {
		repo.decisions[d.ID] = d
	}
	repo.children["target"] = []*domain.Decision{childA}
	repo.children["childA"] = []*domain.Decision{grandchild}
	svc := newTestDecisionService(repo)

	chain, err := svc.CausalChain(context.Background(), "target", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := map[string]*domain.CausalNode{}
	for _, n := range chain {
		byID[n.Decision.ID] = n
	}
	if len(chain) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(chain))
	}
	if n := byID["target"]; n == nil || n.Relation != "target" || n.Depth != 0 {
		t.Errorf("bad target node: %#v", n)
	}
	if n := byID["mid"]; n == nil || n.Relation != "ancestor" || n.Depth != -1 {
		t.Errorf("bad mid node: %#v", n)
	}
	if n := byID["root"]; n == nil || n.Relation != "ancestor" || n.Depth != -2 {
		t.Errorf("bad root node: %#v", n)
	}
	if n := byID["childA"]; n == nil || n.Relation != "descendant" || n.Depth != 1 {
		t.Errorf("bad childA node: %#v", n)
	}
	if n := byID["grandchild"]; n == nil || n.Relation != "descendant" || n.Depth != 2 {
		t.Errorf("bad grandchild node: %#v", n)
	}
}

func TestCausalChain_MaxDepthLimitsDescendants(t *testing.T) {
	repo := newMockDecisionRepo()
	target := &domain.Decision{ID: "t"}
	c1 := &domain.Decision{ID: "c1", ParentDecisionID: "t"}
	c2 := &domain.Decision{ID: "c2", ParentDecisionID: "c1"}
	for _, d := range []*domain.Decision{target, c1, c2} {
		repo.decisions[d.ID] = d
	}
	repo.children["t"] = []*domain.Decision{c1}
	repo.children["c1"] = []*domain.Decision{c2}
	svc := newTestDecisionService(repo)

	chain, err := svc.CausalChain(context.Background(), "t", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range chain {
		if n.Decision.ID == "c2" {
			t.Error("c2 is beyond maxDepth=1 and must not appear")
		}
	}
	if len(chain) != 2 {
		t.Errorf("expected target + c1, got %d nodes", len(chain))
	}
}

func TestCausalChain_TargetNotFound(t *testing.T) {
	svc := newTestDecisionService(newMockDecisionRepo())
	if _, err := svc.CausalChain(context.Background(), "nope", 3); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestAnalyzeInfluence_Aggregates(t *testing.T) {
	repo := newMockDecisionRepo()
	target := &domain.Decision{ID: "t", Category: "deploy", Outcome: domain.DecisionApproved}
	agree := &domain.Decision{ID: "a", ParentDecisionID: "t", Outcome: domain.DecisionApproved}
	disagree := &domain.Decision{ID: "b", ParentDecisionID: "t", Outcome: domain.DecisionRejected, SupersededBy: "x"}
	for _, d := range []*domain.Decision{target, agree, disagree} {
		repo.decisions[d.ID] = d
	}
	repo.children["t"] = []*domain.Decision{agree, disagree}
	repo.countByCat = map[string]int{"deploy": 7}
	svc := newTestDecisionService(repo)

	report, err := svc.AnalyzeInfluence(context.Background(), "t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Descendants != 2 {
		t.Errorf("descendants: got %d, want 2", report.Descendants)
	}
	if report.MaxDepth != 1 {
		t.Errorf("maxDepth: got %d, want 1", report.MaxDepth)
	}
	if report.AgreementRate != 0.5 {
		t.Errorf("agreementRate: got %f, want 0.5", report.AgreementRate)
	}
	if report.SupersedeCount != 1 {
		t.Errorf("supersedeCount: got %d, want 1", report.SupersedeCount)
	}
	if report.CategoryEchoes != 7 {
		t.Errorf("categoryEchoes: got %d, want 7", report.CategoryEchoes)
	}
}

func TestDecisionSupersede_Passthrough(t *testing.T) {
	repo := newMockDecisionRepo()
	svc := newTestDecisionService(repo)

	if err := svc.Supersede(context.Background(), "old", "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.superseded) != 1 || repo.superseded[0] != [2]string{"old", "new"} {
		t.Errorf("unexpected supersede calls: %#v", repo.superseded)
	}
}

func TestBuildDecisionText_SkipsEmptyParts(t *testing.T) {
	got := buildDecisionText(&domain.Decision{
		Category: "deploy",
		Scenario: "  ",
		Outcome:  domain.DecisionApproved,
	})
	want := "deploy | approved"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
