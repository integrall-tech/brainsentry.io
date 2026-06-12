package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
)

// SemanticAPIService provides high-level "remember/recall/improve/forget"
// operations that wrap MemoryService, search, auto-forget, and feedback
// learning. Designed for agent-friendly integration.
type SemanticAPIService struct {
	memoryService   *MemoryService
	autoForgetSvc   *AutoForgetService
	feedbackLearn   *FeedbackLearningService
	nodeSetSvc      *NodeSetService
	traceSvc        *AgentTraceService
	queryRouter     *QueryRouterService
	memoryRepo      memoryRepoForSemanticAPI
}

// memoryRepoForSemanticAPI captures the repo methods we actually need,
// to keep the service decoupled from the concrete postgres.MemoryRepository.
type memoryRepoForSemanticAPI interface {
	FindByID(ctx context.Context, id string) (*domain.Memory, error)
	List(ctx context.Context, page, size int) ([]domain.Memory, int64, error)
}

// NewSemanticAPIService creates a new SemanticAPIService.
func NewSemanticAPIService(
	memoryService *MemoryService,
	autoForgetSvc *AutoForgetService,
	feedbackLearn *FeedbackLearningService,
	nodeSetSvc *NodeSetService,
	traceSvc *AgentTraceService,
	queryRouter *QueryRouterService,
	memoryRepo memoryRepoForSemanticAPI,
) *SemanticAPIService {
	return &SemanticAPIService{
		memoryService: memoryService,
		autoForgetSvc: autoForgetSvc,
		feedbackLearn: feedbackLearn,
		nodeSetSvc:    nodeSetSvc,
		traceSvc:      traceSvc,
		queryRouter:   queryRouter,
		memoryRepo:    memoryRepo,
	}
}

// ---------- remember ----------

// RememberRequest is the agent-friendly input for storing knowledge.
type RememberRequest struct {
	Text       string                  `json:"text"`
	Title      string                  `json:"title,omitempty"`
	SessionID  string                  `json:"sessionId,omitempty"`
	Sets       []string                `json:"sets,omitempty"`
	Tags       []string                `json:"tags,omitempty"`
	Category   domain.MemoryCategory   `json:"category,omitempty"`
	Importance domain.ImportanceLevel  `json:"importance,omitempty"`
}

// RememberResponse returns the created memory ID and metadata.
type RememberResponse struct {
	MemoryID  string   `json:"memoryId"`
	Sets      []string `json:"sets,omitempty"`
	Title     string   `json:"title,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Remember stores new knowledge with simple inputs.
// Wraps CreateMemory and automatically attaches to sets.
func (s *SemanticAPIService) Remember(ctx context.Context, req RememberRequest) (*RememberResponse, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("text is required")
	}

	createReq := dto.CreateMemoryRequest{
		Content:    req.Text,
		Summary:    req.Title,
		Category:   req.Category,
		Importance: req.Importance,
		Tags:       req.Tags,
	}
	if req.Category == "" {
		createReq.Category = domain.CategoryKnowledge
	}
	if req.Importance == "" {
		createReq.Importance = domain.ImportanceImportant
	}

	memory, err := s.memoryService.CreateMemory(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}

	// Attach to sets if provided
	attachedSets := []string{}
	if len(req.Sets) > 0 && s.nodeSetSvc != nil {
		if err := s.nodeSetSvc.AddToSet(ctx, memory.ID, req.Sets...); err != nil {
			// Non-fatal
			return &RememberResponse{
				MemoryID:  memory.ID,
				Title:     memory.Summary,
				CreatedAt: memory.CreatedAt,
			}, nil
		}
		for _, name := range req.Sets {
			attachedSets = append(attachedSets, normalizeSetName(name))
		}
	}

	return &RememberResponse{
		MemoryID:  memory.ID,
		Sets:      attachedSets,
		Title:     memory.Summary,
		CreatedAt: memory.CreatedAt,
	}, nil
}

// ---------- recall ----------

// RecallRequest is the agent-friendly input for querying knowledge.
type RecallRequest struct {
	Query  string   `json:"query"`
	Set    string   `json:"set,omitempty"`   // filter by set (optional)
	Limit  int      `json:"limit,omitempty"` // default 10
	Tags   []string `json:"tags,omitempty"`
}

// RecallResult is a single recall match with explanation.
type RecallResult struct {
	MemoryID       string    `json:"memoryId"`
	Content        string    `json:"content"`
	Summary        string    `json:"summary,omitempty"`
	Relevance      float64   `json:"relevance"`
	Category       string    `json:"category,omitempty"`
	FeedbackWeight float64   `json:"feedbackWeight"`
	CreatedAt      time.Time `json:"createdAt"`
	Sets           []string  `json:"sets,omitempty"`
}

// RecallResponse holds recall results along with routing metadata.
type RecallResponse struct {
	Query    string          `json:"query"`
	Strategy string          `json:"strategy"`
	Results  []RecallResult  `json:"results"`
	Total    int             `json:"total"`
}

// Recall searches memories and returns them with feedback-blended relevance.
func (s *SemanticAPIService) Recall(ctx context.Context, req RecallRequest) (*RecallResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	// Classify query strategy (even if not used to dispatch, useful for tracing)
	strategy := "HYBRID"
	if s.queryRouter != nil {
		strategy = string(s.queryRouter.Classify(req.Query).Strategy)
	}

	searchReq := dto.SearchRequest{
		Query: req.Query,
		Tags:  req.Tags,
		Limit: limit * 3, // overfetch for set filter + feedback reranking
	}

	resp, err := s.memoryService.SearchMemories(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	results := make([]RecallResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		memory, err := s.memoryRepo.FindByID(ctx, r.ID)
		if err != nil || memory == nil {
			continue
		}

		// Filter by set
		sets := s.nodeSetSvc.GetMemorySets(memory)
		if req.Set != "" && !containsString(sets, normalizeSetName(req.Set)) {
			continue
		}

		// Blend feedback weight into relevance
		relevance := r.RelevanceScore
		fbWeight := 0.5
		if s.feedbackLearn != nil {
			fbWeight = s.feedbackLearn.ComputeWeight(memory)
			relevance = s.feedbackLearn.BlendScore(relevance, fbWeight)
		}

		results = append(results, RecallResult{
			MemoryID:       memory.ID,
			Content:        memory.Content,
			Summary:        memory.Summary,
			Relevance:      relevance,
			Category:       string(memory.Category),
			FeedbackWeight: fbWeight,
			CreatedAt:      memory.CreatedAt,
			Sets:           sets,
		})

		if len(results) >= limit {
			break
		}
	}

	// Re-sort by blended relevance (desc)
	sortRecallResults(results)

	return &RecallResponse{
		Query:    req.Query,
		Strategy: strategy,
		Results:  results,
		Total:    len(results),
	}, nil
}

// sortRecallResults sorts results by relevance descending (stable insertion sort).
func sortRecallResults(rs []RecallResult) {
	for i := 1; i < len(rs); i++ {
		key := rs[i]
		j := i - 1
		for j >= 0 && rs[j].Relevance < key.Relevance {
			rs[j+1] = rs[j]
			j--
		}
		rs[j+1] = key
	}
}

// ---------- improve ----------

// ImproveRequest asks the system to incorporate feedback and run learning.
type ImproveRequest struct {
	SessionID    string `json:"sessionId,omitempty"`
	DryRun       bool   `json:"dryRun,omitempty"`
}

// ImproveResponse summarizes what was improved.
type ImproveResponse struct {
	AutoForgetResult *AutoForgetResult `json:"autoForgetResult,omitempty"`
	Message          string            `json:"message"`
}

// Improve triggers learning routines: auto-forget stale/contradictory memories,
// recompute feedback weights, etc. Called periodically or on-demand.
func (s *SemanticAPIService) Improve(ctx context.Context, req ImproveRequest) (*ImproveResponse, error) {
	out := &ImproveResponse{}

	if s.autoForgetSvc != nil {
		res, err := s.autoForgetSvc.Run(ctx, req.DryRun)
		if err == nil {
			out.AutoForgetResult = res
		}
	}

	out.Message = fmt.Sprintf("improvement cycle completed (dryRun=%v)", req.DryRun)
	return out, nil
}

// ---------- forget ----------

// ForgetRequest identifies what to forget.
type ForgetRequest struct {
	MemoryID string `json:"memoryId,omitempty"`
	Set      string `json:"set,omitempty"`   // forget all in a set
	Query    string `json:"query,omitempty"` // natural-language identification
}

// ForgetResponse summarizes what was deleted.
type ForgetResponse struct {
	DeletedIDs []string `json:"deletedIds"`
	Count      int      `json:"count"`
	Message    string   `json:"message"`
}

// Forget removes memories matching the request.
// Supports single ID, all memories in a set, or natural-language query.
func (s *SemanticAPIService) Forget(ctx context.Context, req ForgetRequest) (*ForgetResponse, error) {
	out := &ForgetResponse{}

	// Direct ID path
	if req.MemoryID != "" {
		if err := s.memoryService.DeleteMemory(ctx, req.MemoryID); err != nil {
			return nil, fmt.Errorf("delete memory: %w", err)
		}
		out.DeletedIDs = []string{req.MemoryID}
		out.Count = 1
		out.Message = "memory deleted"
		return out, nil
	}

	// Query path: find memories matching query, delete them
	if req.Query != "" {
		searchReq := dto.SearchRequest{Query: req.Query, Limit: 50}
		resp, err := s.memoryService.SearchMemories(ctx, searchReq)
		if err != nil {
			return nil, fmt.Errorf("search for forget: %w", err)
		}

		for _, r := range resp.Results {
			if req.Set != "" {
				// Additional set filter
				memory, err := s.memoryRepo.FindByID(ctx, r.ID)
				if err != nil || memory == nil {
					continue
				}
				sets := s.nodeSetSvc.GetMemorySets(memory)
				if !containsString(sets, normalizeSetName(req.Set)) {
					continue
				}
			}

			if err := s.memoryService.DeleteMemory(ctx, r.ID); err == nil {
				out.DeletedIDs = append(out.DeletedIDs, r.ID)
			}
		}

		out.Count = len(out.DeletedIDs)
		out.Message = fmt.Sprintf("deleted %d memories matching query", out.Count)
		return out, nil
	}

	// Set-only path: page through the tenant's memories collecting members
	// of the set, then delete. Two passes on purpose — deleting while
	// paginating shifts the pages under us and skips rows.
	if req.Set != "" {
		want := normalizeSetName(req.Set)
		const pageSize = 200
		var targets []string
		for page := 0; ; page++ {
			memories, _, err := s.memoryRepo.List(ctx, page, pageSize)
			if err != nil {
				return nil, fmt.Errorf("list for forget: %w", err)
			}
			if len(memories) == 0 {
				break
			}
			for i := range memories {
				if containsString(s.nodeSetSvc.GetMemorySets(&memories[i]), want) {
					targets = append(targets, memories[i].ID)
				}
			}
			if len(memories) < pageSize {
				break
			}
		}

		for _, id := range targets {
			if err := s.memoryService.DeleteMemory(ctx, id); err == nil {
				out.DeletedIDs = append(out.DeletedIDs, id)
			}
		}
		out.Count = len(out.DeletedIDs)
		out.Message = fmt.Sprintf("deleted %d memories in set %q", out.Count, req.Set)
		return out, nil
	}

	return nil, fmt.Errorf("one of memoryId, query, or set is required")
}
