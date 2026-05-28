package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/repository/graph"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/internal/security"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

var quickCheckPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bagent\b`),
	regexp.MustCompile(`(?i)\bservice\b`),
	regexp.MustCompile(`(?i)\brepository\b`),
	regexp.MustCompile(`(?i)\bcontroller\b`),
	regexp.MustCompile(`(?i)\bcomponent\b`),
	regexp.MustCompile(`(?i)\bclass\b`),
	regexp.MustCompile(`(?i)\bcreate\b`),
	regexp.MustCompile(`(?i)\bimplement\b`),
	regexp.MustCompile(`(?i)\badd\b`),
	regexp.MustCompile(`(?i)\bfix\b`),
	regexp.MustCompile(`(?i)\bbug\b`),
	regexp.MustCompile(`(?i)\berror\b`),
	regexp.MustCompile(`(?i)\bpattern\b`),
	regexp.MustCompile(`(?i)\bdecision\b`),
	regexp.MustCompile(`(?i)\buse\b`),
}

var errorKeywords = []string{
	"error", "exception", "failed", "failure", "bug",
	"issue", "nullpointer", "runtime", "timeout",
}

// InterceptionService handles prompt interception and context injection.
type InterceptionService struct {
	memoryRepo          *postgres.MemoryRepository
	memoryGraphRepo     *graph.MemoryGraphRepository
	graphRAGRepo        *graph.GraphRAGRepository
	openRouter          *OpenRouterService
	embeddingService    *EmbeddingService
	auditService        *AuditService
	noteRepo            *postgres.NoteRepository
	piiService          *PIIService
	quickCheckEnabled   bool
	deepAnalysisEnabled bool
	relevanceThreshold  float64
	defaultTokenBudget  int
}

// NewInterceptionService creates a new InterceptionService.
func NewInterceptionService(
	memoryRepo *postgres.MemoryRepository,
	memoryGraphRepo *graph.MemoryGraphRepository,
	graphRAGRepo *graph.GraphRAGRepository,
	openRouter *OpenRouterService,
	embeddingService *EmbeddingService,
	auditService *AuditService,
	noteRepo *postgres.NoteRepository,
	quickCheckEnabled, deepAnalysisEnabled bool,
	relevanceThreshold float64,
) *InterceptionService {
	return &InterceptionService{
		memoryRepo:          memoryRepo,
		memoryGraphRepo:     memoryGraphRepo,
		graphRAGRepo:        graphRAGRepo,
		openRouter:          openRouter,
		embeddingService:    embeddingService,
		auditService:        auditService,
		noteRepo:            noteRepo,
		piiService:          NewPIIService(),
		quickCheckEnabled:   quickCheckEnabled,
		deepAnalysisEnabled: deepAnalysisEnabled,
		relevanceThreshold:  relevanceThreshold,
		defaultTokenBudget:  2000,
	}
}

// Intercept performs prompt interception with context injection.
func (s *InterceptionService) Intercept(ctx context.Context, req dto.InterceptRequest) (*dto.InterceptResponse, error) {
	start := time.Now()
	llmCalls := 0

	resp := &dto.InterceptResponse{
		OriginalPrompt: req.Prompt,
		Enhanced:       false,
	}

	// Minimum prompt length
	if len(req.Prompt) < 10 {
		resp.Reasoning = "prompt too short"
		resp.LatencyMs = time.Since(start).Milliseconds()
		return resp, nil
	}

	tenantID := tenant.FromContext(ctx)

	// Phase 1: Quick Check (pattern matching)
	if s.quickCheckEnabled && !req.ForceDeepAnalysis {
		if !s.quickCheck(req.Prompt) {
			resp.Reasoning = "no relevant patterns detected"
			resp.LatencyMs = time.Since(start).Milliseconds()
			return resp, nil
		}
	}

	// Phase 2: Deep Analysis (LLM-based)
	if s.deepAnalysisEnabled && s.openRouter != nil {
		memorySummaries := s.getMemorySummaries(ctx, tenantID)
		if len(memorySummaries) > 0 {
			analysis, err := s.openRouter.AnalyzeRelevance(ctx, req.Prompt, memorySummaries)
			llmCalls++
			if err != nil {
				slog.Warn("deep analysis failed", "error", err)
			} else if !analysis.Relevant || analysis.Confidence < s.relevanceThreshold {
				resp.Reasoning = analysis.Reasoning
				resp.Confidence = analysis.Confidence
				resp.LLMCalls = llmCalls
				resp.LatencyMs = time.Since(start).Milliseconds()
				return resp, nil
			} else {
				resp.Reasoning = analysis.Reasoning
				resp.Confidence = analysis.Confidence
			}
		}
	}

	// Phase 3: Vector Search for Relevant Memories
	var memories []domain.Memory
	if s.embeddingService != nil && s.memoryGraphRepo != nil {
		embedding := s.embeddingService.Embed(req.Prompt)
		ids, _, err := s.memoryGraphRepo.VectorSearch(ctx, embedding, 5, tenantID)
		if err != nil {
			slog.Warn("vector search failed, falling back to text search", "error", err)
		}

		for _, id := range ids {
			m, err := s.memoryRepo.FindByID(ctx, id)
			if err == nil {
				memories = append(memories, *m)
			}
		}
	}

	// GraphRAG enrichment: multi-hop from vector search results
	if s.graphRAGRepo != nil && len(memories) > 0 {
		seedIDs := make([]string, 0, len(memories))
		for _, m := range memories {
			seedIDs = append(seedIDs, m.ID)
		}
		enriched, err := s.graphRAGRepo.EnrichContext(ctx, seedIDs, tenantID)
		if err == nil {
			for _, r := range enriched {
				em, err := s.memoryRepo.FindByID(ctx, r.MemoryID)
				if err == nil {
					memories = append(memories, *em)
				}
			}
		}
	}

	// Fallback to text search if no vector results
	if len(memories) == 0 {
		textResults, err := s.memoryRepo.FullTextSearch(ctx, req.Prompt, 5)
		if err == nil {
			memories = textResults
		}
	}

	// Filter expired and superseded memories
	memories = filterActiveMemories(memories)

	// Filter by importance (only CRITICAL and IMPORTANT, max 3)
	memories = filterByImportance(memories, 3)

	if len(memories) == 0 {
		resp.Reasoning = "no relevant memories found"
		resp.LLMCalls = llmCalls
		resp.LatencyMs = time.Since(start).Milliseconds()
		return resp, nil
	}

	// Phase 4: Check for hindsight notes (error-related)
	var hindsightNotes []domain.HindsightNote
	if s.noteRepo != nil && containsErrorKeywords(req.Prompt) {
		notes, err := s.noteRepo.SearchHindsightNotes(ctx, req.Prompt, tenantID, 3)
		if err == nil {
			hindsightNotes = notes
		}
	}

	// Phase 5: Build Context with token budget enforcement
	tokenBudget := s.defaultTokenBudget
	if req.MaxTokens > 0 {
		tokenBudget = req.MaxTokens
	}
	contextStr := s.formatContextWithBudget(memories, hindsightNotes, tokenBudget)

	// Mask PII before injecting into prompt sent to LLM
	if s.piiService != nil {
		maskedContext, piiSummary := s.piiService.MaskForLLM(contextStr)
		if piiSummary != "" {
			slog.Info("PII masked in context injection", "summary", piiSummary)
		}
		contextStr = maskedContext
	}

	enhancedPrompt := contextStr + "\n\n" + req.Prompt
	tokensInjected := estimateTokens(contextStr)

	// Build response
	resp.Enhanced = true
	resp.EnhancedPrompt = enhancedPrompt
	resp.ContextInjected = contextStr
	resp.TokensInjected = tokensInjected
	resp.LLMCalls = llmCalls
	resp.LatencyMs = time.Since(start).Milliseconds()

	// Memory references
	memRefs := make([]dto.MemoryReference, 0, len(memories))
	memIDs := make([]string, 0, len(memories))
	for _, m := range memories {
		memRefs = append(memRefs, dto.MemoryReference{
			ID:             m.ID,
			Summary:        m.Summary,
			Category:       m.Category,
			Importance:     m.Importance,
			RelevanceScore: m.RelevanceScore(),
			Excerpt:        truncate(m.Content, 200),
		})
		memIDs = append(memIDs, m.ID)

		// Track injection async
		go func(id, tid string) {
			bgCtx := tenant.WithTenant(context.Background(), tid)
			s.memoryRepo.IncrementInjectionCount(bgCtx, id)
		}(m.ID, tenantID)
	}
	resp.MemoriesUsed = memRefs

	// Note references
	if len(hindsightNotes) > 0 {
		noteRefs := make([]dto.NoteReference, 0, len(hindsightNotes))
		for _, n := range hindsightNotes {
			noteRefs = append(noteRefs, dto.NoteReference{
				ID:       n.ID,
				Title:    n.Title,
				Type:     domain.NoteHindsight,
				Severity: n.Severity,
				Excerpt:  truncate(n.ErrorMessage, 200),
			})
		}
		resp.NotesUsed = noteRefs
	}

	// Audit log async
	if s.auditService != nil {
		go s.auditService.LogInterception(
			tenant.WithTenant(context.Background(), tenantID),
			req.UserID, req.SessionID, req.Prompt,
			true, memIDs, int(resp.LatencyMs), resp.Confidence, llmCalls, tokensInjected,
		)
	}

	return resp, nil
}

func (s *InterceptionService) quickCheck(prompt string) bool {
	for _, p := range quickCheckPatterns {
		if p.MatchString(prompt) {
			return true
		}
	}
	return false
}

func (s *InterceptionService) getMemorySummaries(ctx context.Context, tenantID string) []string {
	// Get the 10 most recent memories for the LLM relevance pre-filter.
	// The earlier implementation called FullTextSearch(ctx, "", 10), but an
	// empty tsquery returns 0 rows in Postgres — so deep analysis never
	// fired, /v1/intercept always returned enhanced=false with reasoning
	// "no relevant memories found". Surfaced by the sales-intercept
	// validation scenario.
	_ = tenantID // List reads tenant from ctx via tenant.FromContext
	memories, _, err := s.memoryRepo.List(ctx, 0, 10)
	if err != nil || len(memories) == 0 {
		return nil
	}
	summaries := make([]string, 0, len(memories))
	for _, m := range memories {
		if m.Summary != "" {
			summaries = append(summaries, m.Summary)
		}
	}
	return summaries
}

func filterByImportance(memories []domain.Memory, max int) []domain.Memory {
	var result []domain.Memory
	for _, m := range memories {
		if m.Importance == domain.ImportanceCritical || m.Importance == domain.ImportanceImportant {
			result = append(result, m)
			if len(result) >= max {
				break
			}
		}
	}
	// If no important memories, return up to max from original list
	if len(result) == 0 && len(memories) > 0 {
		if len(memories) > max {
			return memories[:max]
		}
		return memories
	}
	return result
}

func filterActiveMemories(memories []domain.Memory) []domain.Memory {
	now := time.Now()
	var active []domain.Memory
	for _, m := range memories {
		if isInactiveMemory(&m, now) {
			continue
		}
		active = append(active, m)
	}
	return active
}

func isInactiveMemory(m *domain.Memory, now time.Time) bool {
	return IsExpired(m, now) || m.SupersededBy != ""
}

func containsErrorKeywords(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range errorKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func estimateTokens(text string) int {
	return len(text) / 4
}

// formatContextWithBudget builds context respecting a token budget via greedy
// packing. Every untrusted blob is wrapped in <memory> framing and run through
// the prompt-injection sanitizer (security.Sanitize) before injection.
func (s *InterceptionService) formatContextWithBudget(memories []domain.Memory, notes []domain.HindsightNote, tokenBudget int) string {
	var sb strings.Builder
	header := "<system_context>\n" + security.SystemPromptPreamble + "\n\n"
	footer := "</system_context>"
	sb.WriteString(header)
	usedTokens := estimateTokens(header) + estimateTokens(footer)

	matchedAll := make(map[string]int)

	// Pack memories by relevance (already sorted by importance filter)
	for _, m := range memories {
		body := m.Summary
		if body == "" {
			body = truncate(m.Content, 300)
		}
		if m.CodeExample != "" {
			body += fmt.Sprintf("\n```%s\n%s\n```", m.ProgrammingLanguage, truncate(m.CodeExample, 500))
		}
		framed, matched := security.FrameMemoryWithMeta(m.ID, fmt.Sprintf("%s/%s", m.Category, m.Importance), body)
		for _, name := range matched {
			matchedAll[name]++
		}
		entry := framed + "\n\n"

		entryTokens := estimateTokens(entry)
		if usedTokens+entryTokens > tokenBudget {
			break
		}
		sb.WriteString(entry)
		usedTokens += entryTokens
	}

	// Pack notes if budget allows
	if len(notes) > 0 {
		noteHeader := "<!-- Hindsight Notes (Past Issues) -->\n"
		noteHeaderTokens := estimateTokens(noteHeader)
		if usedTokens+noteHeaderTokens < tokenBudget {
			sb.WriteString(noteHeader)
			usedTokens += noteHeaderTokens

			for _, n := range notes {
				body := fmt.Sprintf("[%s] %s", n.Severity, truncate(n.ErrorMessage, 200))
				if n.Resolution != "" {
					body += "\nResolution: " + truncate(n.Resolution, 200)
				}
				framed, matched := security.FrameMemoryWithMeta("note:"+n.ID, "hindsight/"+n.Title, body)
				for _, name := range matched {
					matchedAll[name]++
				}
				noteEntry := framed + "\n"

				noteTokens := estimateTokens(noteEntry)
				if usedTokens+noteTokens > tokenBudget {
					break
				}
				sb.WriteString(noteEntry)
				usedTokens += noteTokens
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(footer)

	if len(matchedAll) > 0 {
		slog.Warn("prompt-injection patterns sanitized in context",
			"counts", matchedAll, "memories", len(memories), "notes", len(notes))
	}

	return sb.String()
}
