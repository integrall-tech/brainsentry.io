package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// FactAction represents the LLM's decision for a fact.
type FactAction string

const (
	FactActionAdd    FactAction = "ADD"
	FactActionUpdate FactAction = "UPDATE"
	FactActionDelete FactAction = "DELETE"
	FactActionNone   FactAction = "NONE"
)

// ExtractedFact represents an atomic fact extracted from content.
type ExtractedFact struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
	Context   string `json:"context,omitempty"`
}

// FactDecision represents the LLM's reconciliation decision for a fact.
type FactDecision struct {
	Fact             ExtractedFact `json:"fact"`
	Action           FactAction    `json:"action"`
	Reason           string        `json:"reason"`
	ExistingMemoryID string        `json:"existingMemoryId,omitempty"`
	MergedContent    string        `json:"mergedContent,omitempty"`
}

// ReconciliationResult summarizes the reconciliation outcome.
type ReconciliationResult struct {
	ExtractedFacts int            `json:"extractedFacts"`
	Decisions      []FactDecision `json:"decisions"`
	Added          int            `json:"added"`
	Updated        int            `json:"updated"`
	Deleted        int            `json:"deleted"`
	Skipped        int            `json:"skipped"`
}

// ReconciliationService handles LLM-driven fact extraction and reconciliation.
type ReconciliationService struct {
	openRouter    *OpenRouterService
	llm           LLMProvider
	memoryRepo    *postgres.MemoryRepository
	memoryService *MemoryService
}

// NewReconciliationService creates a new ReconciliationService.
func NewReconciliationService(
	openRouter *OpenRouterService,
	memoryRepo *postgres.MemoryRepository,
	memoryService *MemoryService,
) *ReconciliationService {
	var llm LLMProvider
	if openRouter != nil {
		llm = NewOpenRouterProvider(openRouter)
	}
	return &ReconciliationService{
		openRouter:    openRouter,
		llm:           llm,
		memoryRepo:    memoryRepo,
		memoryService: memoryService,
	}
}

// NewReconciliationServiceWithLLM creates a ReconciliationService with a generic LLM provider.
func NewReconciliationServiceWithLLM(
	llm LLMProvider,
	memoryRepo *postgres.MemoryRepository,
	memoryService *MemoryService,
) *ReconciliationService {
	return &ReconciliationService{
		llm:           llm,
		memoryRepo:    memoryRepo,
		memoryService: memoryService,
	}
}

// ReconcileFacts extracts atomic facts from content and reconciles with existing memories.
func (s *ReconciliationService) ReconcileFacts(ctx context.Context, content string, sessionID string) (*ReconciliationResult, error) {
	if s.llm == nil {
		return &ReconciliationResult{}, nil
	}

	// Phase 1: Extract atomic facts from content
	facts, err := s.extractFacts(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("extracting facts: %w", err)
	}

	if len(facts) == 0 {
		return &ReconciliationResult{ExtractedFacts: 0}, nil
	}

	// Phase 2: For each fact, find similar existing memories
	result := &ReconciliationResult{
		ExtractedFacts: len(facts),
		Decisions:      make([]FactDecision, 0, len(facts)),
	}

	for _, fact := range facts {
		// Search for existing memories similar to this fact
		searchQuery := fmt.Sprintf("%s %s %s", fact.Subject, fact.Predicate, fact.Object)
		var existingMemories []domain.Memory
		if s.memoryRepo != nil {
			var err error
			existingMemories, err = s.memoryRepo.FullTextSearch(ctx, searchQuery, 5)
			if err != nil {
				slog.Warn("failed to search for existing facts", "error", err)
				existingMemories = nil
			}
		}

		// Phase 3: LLM decides action per fact
		decision, err := s.decideAction(ctx, fact, existingMemories)
		if err != nil {
			slog.Warn("failed to decide action for fact", "error", err, "subject", fact.Subject)
			decision = &FactDecision{Fact: fact, Action: FactActionAdd, Reason: "decision failed, defaulting to ADD"}
		}

		result.Decisions = append(result.Decisions, *decision)

		// Phase 4: Execute the decision
		s.executeDecision(ctx, decision, sessionID)

		switch decision.Action {
		case FactActionAdd:
			result.Added++
		case FactActionUpdate:
			result.Updated++
		case FactActionDelete:
			result.Deleted++
		case FactActionNone:
			result.Skipped++
		}
	}

	return result, nil
}

func (s *ReconciliationService) extractFacts(ctx context.Context, content string) ([]ExtractedFact, error) {
	prompt := fmt.Sprintf(`Extract atomic facts from the following text. Each fact should be a simple subject-predicate-object triple.

RULES:
- Use full names, never pronouns
- Each fact must be self-contained
- Dates in ISO 8601 format
- One fact per statement, do not combine multiple facts

Respond in JSON format only:
{
  "facts": [
    {"subject": "full name", "predicate": "relationship or attribute", "object": "value or entity", "context": "optional surrounding context"}
  ]
}

Text:
%s`, truncate(content, 4000))

	response, err := s.llm.Chat(ctx, []ChatMessage{
		{Role: "system", Content: "You are a fact extraction system. Extract atomic facts as subject-predicate-object triples. Respond with valid JSON only."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Facts []ExtractedFact `json:"facts"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(response)), &result); err != nil {
		return nil, fmt.Errorf("parsing facts: %w", err)
	}
	return result.Facts, nil
}

func (s *ReconciliationService) decideAction(ctx context.Context, fact ExtractedFact, existingMemories []domain.Memory) (*FactDecision, error) {
	if len(existingMemories) == 0 {
		return &FactDecision{
			Fact:   fact,
			Action: FactActionAdd,
			Reason: "no existing similar memories found",
		}, nil
	}

	// Build existing memories context
	var existingContext string
	for i, m := range existingMemories {
		existingContext += fmt.Sprintf("\n[Memory %d, ID=%s]: %s", i+1, m.ID, truncate(m.Content, 300))
	}

	prompt := fmt.Sprintf(`Given a new fact and existing memories, decide what action to take.

New fact:
- Subject: %s
- Predicate: %s
- Object: %s
- Context: %s

Existing memories:%s

Decide ONE action:
- ADD: The fact is genuinely new information not covered by existing memories
- UPDATE: The fact updates or corrects an existing memory (specify which memory ID and provide merged content)
- DELETE: The fact contradicts and invalidates an existing memory (specify which memory ID)
- NONE: The fact is already fully covered by existing memories

Respond in JSON format only:
{
  "action": "ADD|UPDATE|DELETE|NONE",
  "reason": "brief explanation",
  "existingMemoryId": "ID of affected memory if UPDATE or DELETE",
  "mergedContent": "new content for the memory if UPDATE"
}`, fact.Subject, fact.Predicate, fact.Object, fact.Context, existingContext)

	response, err := s.llm.Chat(ctx, []ChatMessage{
		{Role: "system", Content: "You are a fact reconciliation system. Decide how to handle new facts relative to existing memories. Respond with valid JSON only."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	var decision struct {
		Action           string `json:"action"`
		Reason           string `json:"reason"`
		ExistingMemoryID string `json:"existingMemoryId"`
		MergedContent    string `json:"mergedContent"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(response)), &decision); err != nil {
		return &FactDecision{Fact: fact, Action: FactActionAdd, Reason: "parse error, defaulting to ADD"}, nil
	}

	return &FactDecision{
		Fact:             fact,
		Action:           FactAction(decision.Action),
		Reason:           decision.Reason,
		ExistingMemoryID: decision.ExistingMemoryID,
		MergedContent:    decision.MergedContent,
	}, nil
}

func (s *ReconciliationService) executeDecision(ctx context.Context, decision *FactDecision, sessionID string) {
	tenantID := tenant.FromContext(ctx)

	switch decision.Action {
	case FactActionAdd:
		if s.memoryService == nil {
			return
		}
		content := fmt.Sprintf("%s %s %s", decision.Fact.Subject, decision.Fact.Predicate, decision.Fact.Object)
		if decision.Fact.Context != "" {
			content += ". " + decision.Fact.Context
		}
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), tenantID)
			_, err := s.memoryService.CreateMemory(bgCtx, createFactMemoryRequest(content, sessionID))
			if err != nil {
				slog.Warn("failed to create memory from fact", "error", err)
			}
		}()

	case FactActionUpdate:
		if s.memoryService == nil || decision.ExistingMemoryID == "" || decision.MergedContent == "" {
			return
		}
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), tenantID)
			_, err := s.memoryService.UpdateMemory(bgCtx, decision.ExistingMemoryID, updateFactMemoryRequest(decision.MergedContent, decision.Reason))
			if err != nil {
				slog.Warn("failed to update memory from fact reconciliation", "error", err)
			}
		}()

	case FactActionDelete:
		if s.memoryRepo == nil || decision.ExistingMemoryID == "" {
			return
		}
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), tenantID)
			// Supersede rather than hard-delete — preserve history
			if err := s.memoryRepo.SupersedeMemory(bgCtx, decision.ExistingMemoryID, "reconciliation:"+decision.Reason); err != nil {
				slog.Warn("reconciliation: failed to supersede memory", "memoryId", decision.ExistingMemoryID, "error", err)
			}
		}()

	case FactActionNone:
		// No action needed
	}
}

func createFactMemoryRequest(content, sessionID string) createMemoryReq {
	return createMemoryReq{
		Content:    content,
		SourceType: "reconciliation",
		Metadata: map[string]any{
			"source":    "fact_reconciliation",
			"sessionId": sessionID,
		},
	}
}

func updateFactMemoryRequest(content, reason string) updateMemoryReq {
	return updateMemoryReq{
		Content:      content,
		ChangeReason: "fact reconciliation: " + reason,
	}
}

// createMemoryReq is a lightweight request for internal memory creation.
type createMemoryReq = dto.CreateMemoryRequest

// updateMemoryReq is a lightweight request for internal memory update.
type updateMemoryReq = dto.UpdateMemoryRequest
