package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// decisionRepository is the minimal repository surface used by
// DecisionService. *postgres.DecisionRepository satisfies it; tests provide
// in-memory fakes.
type decisionRepository interface {
	Create(ctx context.Context, d *domain.Decision) error
	FindByID(ctx context.Context, id string) (*domain.Decision, error)
	List(ctx context.Context, f postgres.DecisionFilter) ([]*domain.Decision, error)
	Children(ctx context.Context, id string) ([]*domain.Decision, error)
	FindPrecedents(ctx context.Context, category string, embedding []float32, limit int) ([]*domain.DecisionPrecedent, error)
	Supersede(ctx context.Context, oldID, newID string) error
	CountByCategory(ctx context.Context) (map[string]int, error)
}

// DecisionService orchestrates recording decisions, looking up precedents,
// building causal chains, and enforcing policies.
type DecisionService struct {
	repo     decisionRepository
	embedder *EmbeddingService
	audit    *AuditService
	policies *PolicyEngine
	tracker  ProvenanceTracker
}

// NewDecisionService wires dependencies.
func NewDecisionService(repo *postgres.DecisionRepository, embedder *EmbeddingService, audit *AuditService) *DecisionService {
	return &DecisionService{
		repo:     repo,
		embedder: embedder,
		audit:    audit,
		tracker:  NewProvenanceTracker("decision", audit),
	}
}

// WithPolicyEngine attaches a policy engine; enforced on Record.
func (s *DecisionService) WithPolicyEngine(p *PolicyEngine) *DecisionService {
	s.policies = p
	return s
}

// RecordDecisionRequest is the input for Record.
type RecordDecisionRequest struct {
	Category         string                 `json:"category"`
	Scenario         string                 `json:"scenario"`
	Reasoning        string                 `json:"reasoning"`
	Outcome          domain.DecisionOutcome `json:"outcome"`
	Confidence       float64                `json:"confidence"`
	AgentID          string                 `json:"agentId,omitempty"`
	SessionID        string                 `json:"sessionId,omitempty"`
	ParentDecisionID string                 `json:"parentDecisionId,omitempty"`
	EntityIDs        []string               `json:"entityIds,omitempty"`
	MemoryIDs        []string               `json:"memoryIds,omitempty"`
	ValidFrom        *time.Time             `json:"validFrom,omitempty"`
	ValidUntil       *time.Time             `json:"validUntil,omitempty"`
	Metadata         map[string]any         `json:"metadata,omitempty"`
}

// Record persists a new Decision, generating its embedding and enforcing
// policies when the engine is attached.
func (s *DecisionService) Record(ctx context.Context, req RecordDecisionRequest) (*domain.Decision, error) {
	if req.Category == "" || req.Scenario == "" {
		return nil, fmt.Errorf("category and scenario are required")
	}
	if req.Outcome == "" {
		req.Outcome = domain.DecisionPending
	}

	d := &domain.Decision{
		ID:               uuid.NewString(),
		Category:         req.Category,
		Scenario:         req.Scenario,
		Reasoning:        req.Reasoning,
		Outcome:          req.Outcome,
		Confidence:       req.Confidence,
		AgentID:          req.AgentID,
		SessionID:        req.SessionID,
		ParentDecisionID: req.ParentDecisionID,
		EntityIDs:        req.EntityIDs,
		MemoryIDs:        req.MemoryIDs,
		ValidFrom:        req.ValidFrom,
		ValidUntil:       req.ValidUntil,
		CreatedAt:        time.Now(),
		RecordedAt:       time.Now(),
	}

	if len(req.Metadata) > 0 {
		raw, err := json.Marshal(req.Metadata)
		if err == nil {
			d.Metadata = raw
		}
	}

	if s.embedder != nil {
		text := buildDecisionText(d)
		d.Embedding = s.embedder.Embed(text)
	}

	if s.policies != nil {
		violations := s.policies.EvaluateDecision(ctx, d)
		d.PolicyViolations = violations
	}

	if err := s.tracker.Track(ctx, "record", d.ID, func(ctx context.Context) error {
		return s.repo.Create(ctx, d)
	}); err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.LogEvent(ctx, "decision.recorded", map[string]any{
			"decisionId": d.ID,
			"category":   d.Category,
			"outcome":    d.Outcome,
			"violations": len(d.PolicyViolations),
		})
	}

	return d, nil
}

// Get returns a decision by ID.
func (s *DecisionService) Get(ctx context.Context, id string) (*domain.Decision, error) {
	return s.repo.FindByID(ctx, id)
}

// List returns decisions filtered by DecisionFilter.
func (s *DecisionService) List(ctx context.Context, f postgres.DecisionFilter) ([]*domain.Decision, error) {
	return s.repo.List(ctx, f)
}

// FindPrecedentsForCategory returns similar decisions in the same category
// as a freeform scenario text (or explicit decisionID).
func (s *DecisionService) FindPrecedentsForCategory(ctx context.Context, category, scenario string, limit int) ([]*domain.DecisionPrecedent, error) {
	var vec []float32
	if s.embedder != nil && scenario != "" {
		vec = s.embedder.Embed(scenario)
	}
	return s.repo.FindPrecedents(ctx, category, vec, limit)
}

// FindPrecedentsForDecision returns precedents similar to an existing decision.
func (s *DecisionService) FindPrecedentsForDecision(ctx context.Context, id string, limit int) ([]*domain.DecisionPrecedent, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	vec := d.Embedding
	if len(vec) == 0 && s.embedder != nil {
		vec = s.embedder.Embed(buildDecisionText(d))
	}
	precedents, err := s.repo.FindPrecedents(ctx, d.Category, vec, limit+1)
	if err != nil {
		return nil, err
	}
	// drop self
	filtered := make([]*domain.DecisionPrecedent, 0, len(precedents))
	for _, p := range precedents {
		if p.Decision.ID == id {
			continue
		}
		filtered = append(filtered, p)
		if len(filtered) >= limit {
			break
		}
	}
	return filtered, nil
}

// CausalChain walks up to the root ancestor and down through children to
// return a full causal graph around a decision.
func (s *DecisionService) CausalChain(ctx context.Context, id string, maxDepth int) ([]*domain.CausalNode, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	target, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	chain := []*domain.CausalNode{{Decision: target, Depth: 0, Relation: "target"}}

	// ancestors
	current := target
	depth := -1
	for i := 0; i < maxDepth && current.ParentDecisionID != ""; i++ {
		parent, err := s.repo.FindByID(ctx, current.ParentDecisionID)
		if err != nil {
			break
		}
		chain = append(chain, &domain.CausalNode{Decision: parent, Depth: depth, Relation: "ancestor"})
		current = parent
		depth--
	}

	// descendants (BFS)
	type queued struct {
		id    string
		depth int
	}
	queue := []queued{{id: target.ID, depth: 0}}
	visited := map[string]bool{target.ID: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		children, err := s.repo.Children(ctx, cur.id)
		if err != nil {
			continue
		}
		for _, child := range children {
			if visited[child.ID] {
				continue
			}
			visited[child.ID] = true
			chain = append(chain, &domain.CausalNode{Decision: child, Depth: cur.depth + 1, Relation: "descendant"})
			queue = append(queue, queued{id: child.ID, depth: cur.depth + 1})
		}
	}

	return chain, nil
}

// AnalyzeInfluence returns aggregate stats about how a decision propagated.
type InfluenceReport struct {
	Descendants     int     `json:"descendants"`
	MaxDepth        int     `json:"maxDepth"`
	SupersedeCount  int     `json:"supersedeCount"`
	AgreementRate   float64 `json:"agreementRate"`
	CategoryEchoes  int     `json:"categoryEchoes"`
}

// AnalyzeInfluence summarises how a decision echoed through later decisions.
func (s *DecisionService) AnalyzeInfluence(ctx context.Context, id string) (*InfluenceReport, error) {
	target, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	chain, err := s.CausalChain(ctx, id, 10)
	if err != nil {
		return nil, err
	}
	report := &InfluenceReport{}
	var sameOutcome int
	for _, node := range chain {
		if node.Relation != "descendant" {
			continue
		}
		report.Descendants++
		if node.Depth > report.MaxDepth {
			report.MaxDepth = node.Depth
		}
		if node.Decision.Outcome == target.Outcome {
			sameOutcome++
		}
		if node.Decision.SupersededBy != "" {
			report.SupersedeCount++
		}
	}
	if report.Descendants > 0 {
		report.AgreementRate = float64(sameOutcome) / float64(report.Descendants)
	}

	byCat, err := s.repo.CountByCategory(ctx)
	if err == nil {
		report.CategoryEchoes = byCat[target.Category]
	}

	return report, nil
}

// Supersede marks oldID as replaced by newID.
func (s *DecisionService) Supersede(ctx context.Context, oldID, newID string) error {
	return s.repo.Supersede(ctx, oldID, newID)
}

func buildDecisionText(d *domain.Decision) string {
	parts := []string{d.Category, d.Scenario, d.Reasoning, string(d.Outcome)}
	return strings.Join(trimEmpty(parts), " | ")
}

func trimEmpty(s []string) []string {
	out := s[:0]
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
