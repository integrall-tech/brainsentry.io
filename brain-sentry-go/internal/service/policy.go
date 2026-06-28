package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// PolicyEngine evaluates Policies against Decisions. Evaluation is in-process
// with no network hop; rules are loaded from Postgres lazily and filtered by
// the decision's category so only relevant policies fire.
// policyRepository is the minimal repository surface used by PolicyEngine.
// *postgres.PolicyRepository satisfies it; tests provide in-memory fakes.
type policyRepository interface {
	Create(ctx context.Context, p *domain.Policy) error
	Update(ctx context.Context, p *domain.Policy) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*domain.Policy, error)
	ListForCategory(ctx context.Context, category string) ([]*domain.Policy, error)
	ListAll(ctx context.Context) ([]*domain.Policy, error)
}

type PolicyEngine struct {
	repo  policyRepository
	audit *AuditService
}

// NewPolicyEngine wires the repository.
func NewPolicyEngine(repo *postgres.PolicyRepository, audit *AuditService) *PolicyEngine {
	return &PolicyEngine{repo: repo, audit: audit}
}

// CreatePolicyRequest is the input for Create.
type CreatePolicyRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Category    string                `json:"category"`
	Severity    domain.PolicySeverity `json:"severity"`
	RuleType    domain.PolicyRuleType `json:"ruleType"`
	RuleConfig  map[string]any        `json:"ruleConfig"`
	Enabled     *bool                 `json:"enabled,omitempty"`
}

// Create adds a new policy.
func (e *PolicyEngine) Create(ctx context.Context, req CreatePolicyRequest) (*domain.Policy, error) {
	if req.Name == "" || req.Category == "" || req.RuleType == "" {
		return nil, fmt.Errorf("name, category, and ruleType are required")
	}
	raw, _ := json.Marshal(req.RuleConfig)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	severity := req.Severity
	if severity == "" {
		severity = domain.PolicyWarning
	}
	p := &domain.Policy{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Severity:    severity,
		RuleType:    req.RuleType,
		RuleConfig:  raw,
		Enabled:     enabled,
	}
	if err := e.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update modifies an existing policy.
func (e *PolicyEngine) Update(ctx context.Context, id string, req CreatePolicyRequest) (*domain.Policy, error) {
	p, err := e.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Category != "" {
		p.Category = req.Category
	}
	if req.Severity != "" {
		p.Severity = req.Severity
	}
	if req.RuleType != "" {
		p.RuleType = req.RuleType
	}
	if req.RuleConfig != nil {
		raw, _ := json.Marshal(req.RuleConfig)
		p.RuleConfig = raw
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if err := e.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a policy.
func (e *PolicyEngine) Delete(ctx context.Context, id string) error {
	return e.repo.Delete(ctx, id)
}

// Get returns a single policy.
func (e *PolicyEngine) Get(ctx context.Context, id string) (*domain.Policy, error) {
	return e.repo.FindByID(ctx, id)
}

// List returns all policies for the tenant.
func (e *PolicyEngine) List(ctx context.Context) ([]*domain.Policy, error) {
	return e.repo.ListAll(ctx)
}

// EvaluateDecision runs all applicable policies against a decision and
// returns the list of violation identifiers. The detailed report is produced
// by ExplainDecision.
func (e *PolicyEngine) EvaluateDecision(ctx context.Context, d *domain.Decision) []string {
	report := e.ExplainDecision(ctx, d)
	ids := make([]string, 0, len(report))
	for _, v := range report {
		ids = append(ids, v.PolicyID)
	}
	return ids
}

// ExplainDecision returns every violation with its policy metadata.
func (e *PolicyEngine) ExplainDecision(ctx context.Context, d *domain.Decision) []domain.PolicyViolation {
	if e == nil || e.repo == nil {
		return nil
	}
	policies, err := e.repo.ListForCategory(ctx, d.Category)
	if err != nil {
		return nil
	}
	var violations []domain.PolicyViolation
	for _, p := range policies {
		if msg, violated := e.evaluate(p, d); violated {
			violations = append(violations, domain.PolicyViolation{
				PolicyID:   p.ID,
				PolicyName: p.Name,
				Severity:   p.Severity,
				Message:    msg,
			})
		}
	}

	if e.audit != nil && len(violations) > 0 {
		_ = e.audit.LogEvent(ctx, "policy.violated", map[string]any{
			"decisionId": d.ID,
			"count":      len(violations),
			"timestamp":  time.Now().UTC(),
		})
	}

	return violations
}

// EnforceByID fetches the decision by ID and evaluates applicable policies.
type EnforceReport struct {
	Decision   *domain.Decision          `json:"decision"`
	Violations []domain.PolicyViolation `json:"violations"`
	Compliant  bool                      `json:"compliant"`
}

// EnforceByID is used by the /v1/policies/enforce endpoint.
func (e *PolicyEngine) EnforceByID(ctx context.Context, decisionRepo *postgres.DecisionRepository, decisionID string) (*EnforceReport, error) {
	d, err := decisionRepo.FindByID(ctx, decisionID)
	if err != nil {
		return nil, err
	}
	violations := e.ExplainDecision(ctx, d)
	return &EnforceReport{
		Decision:   d,
		Violations: violations,
		Compliant:  len(violations) == 0,
	}, nil
}

// evaluate applies the rule for a single policy.
func (e *PolicyEngine) evaluate(p *domain.Policy, d *domain.Decision) (string, bool) {
	var cfg map[string]any
	if len(p.RuleConfig) > 0 {
		_ = json.Unmarshal(p.RuleConfig, &cfg)
	}

	switch p.RuleType {
	case domain.PolicyMinConfidence:
		threshold, _ := floatFromAny(cfg["threshold"])
		if d.Confidence < threshold {
			return fmt.Sprintf("confidence %.2f below required %.2f", d.Confidence, threshold), true
		}
	case domain.PolicyRequiresMemory:
		min := intFromAny(cfg["min"])
		if min == 0 {
			min = 1
		}
		if len(d.MemoryIDs) < min {
			return fmt.Sprintf("requires at least %d memory citations, got %d", min, len(d.MemoryIDs)), true
		}
	case domain.PolicyRequiresEntity:
		min := intFromAny(cfg["min"])
		if min == 0 {
			min = 1
		}
		if len(d.EntityIDs) < min {
			return fmt.Sprintf("requires at least %d linked entities, got %d", min, len(d.EntityIDs)), true
		}
	case domain.PolicyForbiddenOutcome:
		if outcomes, ok := cfg["outcomes"].([]any); ok {
			for _, o := range outcomes {
				if s, ok := o.(string); ok && strings.EqualFold(s, string(d.Outcome)) {
					return fmt.Sprintf("outcome %q is forbidden", d.Outcome), true
				}
			}
		}
	case domain.PolicyRequiresReasoning:
		min := intFromAny(cfg["minLength"])
		if min == 0 {
			min = 20
		}
		if len(strings.TrimSpace(d.Reasoning)) < min {
			return fmt.Sprintf("reasoning must be at least %d characters", min), true
		}
	case domain.PolicyCategoryBlocked:
		return fmt.Sprintf("category %q is blocked", d.Category), true
	}
	return "", false
}

func floatFromAny(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	}
	return 0
}
