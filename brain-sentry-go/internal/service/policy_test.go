package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
)

// mockPolicyRepo is an in-memory policyRepository for tests.
type mockPolicyRepo struct {
	policies   map[string]*domain.Policy
	byCategory map[string][]*domain.Policy

	created []*domain.Policy
	updated []*domain.Policy
	deleted []string

	createErr error
	updateErr error
	listErr   error
}

func newMockPolicyRepo() *mockPolicyRepo {
	return &mockPolicyRepo{
		policies:   map[string]*domain.Policy{},
		byCategory: map[string][]*domain.Policy{},
	}
}

func (m *mockPolicyRepo) Create(ctx context.Context, p *domain.Policy) error {
	if m.createErr != nil {
		return m.createErr
	}
	if p.ID == "" {
		p.ID = "pol-generated"
	}
	m.created = append(m.created, p)
	m.policies[p.ID] = p
	return nil
}

func (m *mockPolicyRepo) Update(ctx context.Context, p *domain.Policy) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = append(m.updated, p)
	m.policies[p.ID] = p
	return nil
}

func (m *mockPolicyRepo) Delete(ctx context.Context, id string) error {
	m.deleted = append(m.deleted, id)
	delete(m.policies, id)
	return nil
}

func (m *mockPolicyRepo) FindByID(ctx context.Context, id string) (*domain.Policy, error) {
	p, ok := m.policies[id]
	if !ok {
		return nil, errors.New("policy not found")
	}
	return p, nil
}

func (m *mockPolicyRepo) ListForCategory(ctx context.Context, category string) ([]*domain.Policy, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.byCategory[category], nil
}

func (m *mockPolicyRepo) ListAll(ctx context.Context) ([]*domain.Policy, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]*domain.Policy, 0, len(m.policies))
	for _, p := range m.policies {
		out = append(out, p)
	}
	return out, nil
}

func mustJSON(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal rule config: %v", err)
	}
	return raw
}

func TestPolicyCreate_HappyPathDefaults(t *testing.T) {
	repo := newMockPolicyRepo()
	engine := &PolicyEngine{repo: repo}

	p, err := engine.Create(context.Background(), CreatePolicyRequest{
		Name:       "needs evidence",
		Category:   "deploy",
		RuleType:   domain.PolicyRequiresMemory,
		RuleConfig: map[string]any{"min": 2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Enabled {
		t.Error("expected enabled to default to true")
	}
	if p.Severity != domain.PolicyWarning {
		t.Errorf("expected default severity warning, got %s", p.Severity)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 create, got %d", len(repo.created))
	}
	var cfg map[string]any
	if err := json.Unmarshal(p.RuleConfig, &cfg); err != nil || cfg["min"] != float64(2) {
		t.Errorf("rule config not persisted: %s (err=%v)", p.RuleConfig, err)
	}
}

func TestPolicyCreate_ExplicitEnabledAndSeverity(t *testing.T) {
	engine := &PolicyEngine{repo: newMockPolicyRepo()}
	disabled := false

	p, err := engine.Create(context.Background(), CreatePolicyRequest{
		Name:     "blocker",
		Category: "deploy",
		RuleType: domain.PolicyCategoryBlocked,
		Severity: domain.PolicyCritical,
		Enabled:  &disabled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Enabled {
		t.Error("expected enabled=false to be honoured")
	}
	if p.Severity != domain.PolicyCritical {
		t.Errorf("expected critical severity, got %s", p.Severity)
	}
}

func TestPolicyCreate_MissingRequiredFields(t *testing.T) {
	repo := newMockPolicyRepo()
	engine := &PolicyEngine{repo: repo}

	cases := []CreatePolicyRequest{
		{},
		{Name: "x", Category: "deploy"},                       // missing ruleType
		{Name: "x", RuleType: domain.PolicyMinConfidence},     // missing category
		{Category: "deploy", RuleType: domain.PolicyMinConfidence}, // missing name
	}
	for i, req := range cases {
		if _, err := engine.Create(context.Background(), req); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
	if len(repo.created) != 0 {
		t.Errorf("nothing should be persisted on validation error, got %d", len(repo.created))
	}
}

func TestPolicyCreate_RepoError(t *testing.T) {
	repo := newMockPolicyRepo()
	repo.createErr = errors.New("db down")
	engine := &PolicyEngine{repo: repo}

	if _, err := engine.Create(context.Background(), CreatePolicyRequest{
		Name: "x", Category: "c", RuleType: domain.PolicyMinConfidence,
	}); err == nil {
		t.Fatal("expected repo error to propagate")
	}
}

func TestPolicyUpdate_PartialFields(t *testing.T) {
	repo := newMockPolicyRepo()
	repo.policies["p1"] = &domain.Policy{
		ID:       "p1",
		Name:     "old name",
		Category: "deploy",
		Severity: domain.PolicyWarning,
		RuleType: domain.PolicyMinConfidence,
		Enabled:  true,
	}
	engine := &PolicyEngine{repo: repo}
	disabled := false

	p, err := engine.Update(context.Background(), "p1", CreatePolicyRequest{
		Name:    "new name",
		Enabled: &disabled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "new name" {
		t.Errorf("name not updated: %q", p.Name)
	}
	if p.Category != "deploy" || p.Severity != domain.PolicyWarning || p.RuleType != domain.PolicyMinConfidence {
		t.Error("untouched fields must be preserved")
	}
	if p.Enabled {
		t.Error("enabled=false not applied")
	}
	if len(repo.updated) != 1 {
		t.Errorf("expected 1 update call, got %d", len(repo.updated))
	}
}

func TestPolicyUpdate_NotFound(t *testing.T) {
	engine := &PolicyEngine{repo: newMockPolicyRepo()}
	if _, err := engine.Update(context.Background(), "missing", CreatePolicyRequest{Name: "x"}); err == nil {
		t.Fatal("expected error for unknown policy")
	}
}

func TestExplainDecision_RuleTypes(t *testing.T) {
	mkPolicy := func(id string, rt domain.PolicyRuleType, cfg map[string]any) *domain.Policy {
		p := &domain.Policy{ID: id, Name: id, Category: "deploy", Severity: domain.PolicyError, RuleType: rt}
		if cfg != nil {
			p.RuleConfig = mustJSON(t, cfg)
		}
		return p
	}

	cases := []struct {
		name     string
		policy   *domain.Policy
		decision *domain.Decision
		violated bool
		msgPart  string
	}{
		{
			name:     "min confidence below threshold",
			policy:   mkPolicy("p1", domain.PolicyMinConfidence, map[string]any{"threshold": 0.8}),
			decision: &domain.Decision{Category: "deploy", Confidence: 0.5},
			violated: true,
			msgPart:  "confidence",
		},
		{
			name:     "min confidence satisfied",
			policy:   mkPolicy("p1", domain.PolicyMinConfidence, map[string]any{"threshold": 0.8}),
			decision: &domain.Decision{Category: "deploy", Confidence: 0.9},
			violated: false,
		},
		{
			name:     "requires memory defaults to 1",
			policy:   mkPolicy("p2", domain.PolicyRequiresMemory, nil),
			decision: &domain.Decision{Category: "deploy"},
			violated: true,
			msgPart:  "memory citations",
		},
		{
			name:     "requires memory satisfied",
			policy:   mkPolicy("p2", domain.PolicyRequiresMemory, nil),
			decision: &domain.Decision{Category: "deploy", MemoryIDs: []string{"m1"}},
			violated: false,
		},
		{
			name:     "requires entity with explicit min",
			policy:   mkPolicy("p3", domain.PolicyRequiresEntity, map[string]any{"min": 2}),
			decision: &domain.Decision{Category: "deploy", EntityIDs: []string{"e1"}},
			violated: true,
			msgPart:  "linked entities",
		},
		{
			name:     "forbidden outcome case-insensitive",
			policy:   mkPolicy("p4", domain.PolicyForbiddenOutcome, map[string]any{"outcomes": []any{"APPROVED"}}),
			decision: &domain.Decision{Category: "deploy", Outcome: domain.DecisionApproved},
			violated: true,
			msgPart:  "forbidden",
		},
		{
			name:     "forbidden outcome not matching",
			policy:   mkPolicy("p4", domain.PolicyForbiddenOutcome, map[string]any{"outcomes": []any{"rejected"}}),
			decision: &domain.Decision{Category: "deploy", Outcome: domain.DecisionApproved},
			violated: false,
		},
		{
			name:     "requires reasoning default 20 chars",
			policy:   mkPolicy("p5", domain.PolicyRequiresReasoning, nil),
			decision: &domain.Decision{Category: "deploy", Reasoning: "too short   "},
			violated: true,
			msgPart:  "reasoning",
		},
		{
			name:     "requires reasoning satisfied",
			policy:   mkPolicy("p5", domain.PolicyRequiresReasoning, nil),
			decision: &domain.Decision{Category: "deploy", Reasoning: "this reasoning is definitely long enough"},
			violated: false,
		},
		{
			name:     "category blocked always violates",
			policy:   mkPolicy("p6", domain.PolicyCategoryBlocked, nil),
			decision: &domain.Decision{Category: "deploy"},
			violated: true,
			msgPart:  "blocked",
		},
		{
			name:     "unknown rule type never violates",
			policy:   mkPolicy("p7", domain.PolicyRuleType("bogus"), nil),
			decision: &domain.Decision{Category: "deploy"},
			violated: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockPolicyRepo()
			repo.byCategory["deploy"] = []*domain.Policy{tc.policy}
			engine := &PolicyEngine{repo: repo}

			violations := engine.ExplainDecision(context.Background(), tc.decision)
			if tc.violated {
				if len(violations) != 1 {
					t.Fatalf("expected 1 violation, got %d", len(violations))
				}
				v := violations[0]
				if v.PolicyID != tc.policy.ID || v.Severity != tc.policy.Severity {
					t.Errorf("violation metadata mismatch: %#v", v)
				}
				if !strings.Contains(v.Message, tc.msgPart) {
					t.Errorf("message %q missing %q", v.Message, tc.msgPart)
				}
			} else if len(violations) != 0 {
				t.Errorf("expected no violations, got %#v", violations)
			}
		})
	}
}

func TestExplainDecision_NilEngineAndNilRepo(t *testing.T) {
	var nilEngine *PolicyEngine
	if got := nilEngine.ExplainDecision(context.Background(), &domain.Decision{Category: "x"}); got != nil {
		t.Errorf("nil engine must return nil, got %#v", got)
	}
	engine := &PolicyEngine{}
	if got := engine.ExplainDecision(context.Background(), &domain.Decision{Category: "x"}); got != nil {
		t.Errorf("nil repo must return nil, got %#v", got)
	}
}

func TestExplainDecision_RepoListError(t *testing.T) {
	repo := newMockPolicyRepo()
	repo.listErr = errors.New("db down")
	engine := &PolicyEngine{repo: repo}

	if got := engine.ExplainDecision(context.Background(), &domain.Decision{Category: "deploy"}); got != nil {
		t.Errorf("list error must yield nil violations, got %#v", got)
	}
}

func TestEvaluateDecision_ReturnsPolicyIDs(t *testing.T) {
	repo := newMockPolicyRepo()
	repo.byCategory["deploy"] = []*domain.Policy{
		{ID: "p1", Name: "blocked", Category: "deploy", RuleType: domain.PolicyCategoryBlocked},
		{ID: "p2", Name: "evidence", Category: "deploy", RuleType: domain.PolicyRequiresMemory},
	}
	engine := &PolicyEngine{repo: repo}

	ids := engine.EvaluateDecision(context.Background(), &domain.Decision{Category: "deploy"})
	if len(ids) != 2 || ids[0] != "p1" || ids[1] != "p2" {
		t.Errorf("unexpected ids: %#v", ids)
	}
}

func TestPolicyDeleteAndGetAndList(t *testing.T) {
	repo := newMockPolicyRepo()
	repo.policies["p1"] = &domain.Policy{ID: "p1", Name: "a"}
	engine := &PolicyEngine{repo: repo}

	if p, err := engine.Get(context.Background(), "p1"); err != nil || p.Name != "a" {
		t.Errorf("get failed: %v %#v", err, p)
	}
	if list, err := engine.List(context.Background()); err != nil || len(list) != 1 {
		t.Errorf("list failed: %v len=%d", err, len(list))
	}
	if err := engine.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "p1" {
		t.Errorf("delete not forwarded: %#v", repo.deleted)
	}
}
