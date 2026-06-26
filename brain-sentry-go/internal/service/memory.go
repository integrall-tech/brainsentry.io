package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	graphrepo "github.com/integraltech/brainsentry/internal/repository/graph"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// MemoryService handles memory business logic.
type MemoryService struct {
	memoryRepo       memoryRepository
	versionRepo      *postgres.VersionRepository
	memoryGraphRepo  memoryGraphRepository
	auditService     *AuditService
	openRouter       *OpenRouterService
	embeddingService embeddingGenerator
	piiService       *PIIService
	autoImportance   bool

	// Optional pipeline enhancers (set via With* methods). When nil, behavior is unchanged.
	compressor     *MemoryCompressionService     // extracts facts/concepts on create
	queryExpander  *QueryExpansionService        // expands search queries
	stripper       *PrivacyStrippingService      // strips secrets before storage
	tripletSvc     *TripletExtractionService     // extracts S-P-O triplets
	stalenessSvc   *CascadingStalenessService    // propagates staleness on supersede
	feedbackSvc    *FeedbackLearningService      // blends feedback into scoring
	eventSvc       *EventService                 // async event extraction on create
}

// WithCompressor enables LLM-driven content compression during CreateMemory.
func (s *MemoryService) WithCompressor(c *MemoryCompressionService) *MemoryService {
	s.compressor = c
	return s
}

// WithQueryExpander enables LLM-based query reformulation during SearchMemories.
func (s *MemoryService) WithQueryExpander(q *QueryExpansionService) *MemoryService {
	s.queryExpander = q
	return s
}

// WithPrivacyStripper enables secret/PII stripping before storage.
func (s *MemoryService) WithPrivacyStripper(p *PrivacyStrippingService) *MemoryService {
	s.stripper = p
	return s
}

// WithTripletExtractor enables background triplet extraction after CreateMemory.
func (s *MemoryService) WithTripletExtractor(t *TripletExtractionService) *MemoryService {
	s.tripletSvc = t
	return s
}

// WithCascadingStaleness enables BFS staleness propagation after SupersedeMemory.
func (s *MemoryService) WithCascadingStaleness(c *CascadingStalenessService) *MemoryService {
	s.stalenessSvc = c
	return s
}

// WithFeedbackLearning enables feedback-weighted scoring during SearchMemories.
func (s *MemoryService) WithFeedbackLearning(f *FeedbackLearningService) *MemoryService {
	s.feedbackSvc = f
	return s
}

// WithEventExtractor enables async LLM-driven event extraction after CreateMemory.
// The extraction runs in a detached goroutine so it never blocks the create path.
func (s *MemoryService) WithEventExtractor(e *EventService) *MemoryService {
	s.eventSvc = e
	return s
}

type memoryRepository interface {
	Create(ctx context.Context, m *domain.Memory) error
	FindByID(ctx context.Context, id string) (*domain.Memory, error)
	List(ctx context.Context, page, size int) ([]domain.Memory, int64, error)
	Update(ctx context.Context, m *domain.Memory) error
	Delete(ctx context.Context, id string) error
	FindByCategory(ctx context.Context, category domain.MemoryCategory) ([]domain.Memory, error)
	FindByImportance(ctx context.Context, importance domain.ImportanceLevel) ([]domain.Memory, error)
	FullTextSearch(ctx context.Context, query string, limit int) ([]domain.Memory, error)
	IncrementAccessCount(ctx context.Context, id string) error
	RecordFeedback(ctx context.Context, id string, helpful bool) error
	FindSimHashes(ctx context.Context) (map[string]string, error)
	BoostAccessCount(ctx context.Context, id string, boost int) error
	SupersedeMemory(ctx context.Context, oldID, newID string) error
}

type memoryGraphRepository interface {
	VectorSearch(ctx context.Context, embedding []float32, limit int, tenantID string) ([]string, []float64, error)
}

type embeddingGenerator interface {
	Embed(text string) []float32
	HasAPI() bool
}

// NewMemoryService creates a new MemoryService.
func NewMemoryService(
	memoryRepo *postgres.MemoryRepository,
	versionRepo *postgres.VersionRepository,
	memoryGraphRepo *graphrepo.MemoryGraphRepository,
	auditService *AuditService,
	openRouter *OpenRouterService,
	embeddingService *EmbeddingService,
	autoImportance bool,
) *MemoryService {
	var graphRepo memoryGraphRepository
	if memoryGraphRepo != nil {
		graphRepo = memoryGraphRepo
	}
	var embeddings embeddingGenerator
	if embeddingService != nil {
		embeddings = embeddingService
	}
	return &MemoryService{
		memoryRepo:       memoryRepo,
		versionRepo:      versionRepo,
		memoryGraphRepo:  graphRepo,
		auditService:     auditService,
		openRouter:       openRouter,
		embeddingService: embeddings,
		piiService:       NewPIIService(),
		autoImportance:   autoImportance,
	}
}

// CreateMemory creates a new memory with auto-analysis and embedding generation.
func (s *MemoryService) CreateMemory(ctx context.Context, req dto.CreateMemoryRequest) (*domain.Memory, error) {
	// 1. Privacy stripping — strip secrets/PII before anything else touches the content.
	if s.stripper != nil {
		req.Content = s.stripper.StripBeforeStorage(req.Content)
		if req.CodeExample != "" {
			req.CodeExample = s.stripper.StripBeforeStorage(req.CodeExample)
		}
	}

	m := &domain.Memory{
		Content:             req.Content,
		Summary:             req.Summary,
		Category:            req.Category,
		Importance:          req.Importance,
		MemoryType:          req.MemoryType,
		Tags:                req.Tags,
		SourceType:          req.SourceType,
		SourceReference:     req.SourceReference,
		CodeExample:         req.CodeExample,
		ProgrammingLanguage: req.ProgrammingLanguage,
		CreatedBy:           req.CreatedBy,
	}

	// Set emotional weight if provided
	if req.EmotionalWeight != nil {
		w := *req.EmotionalWeight
		if w < -1 {
			w = -1
		}
		if w > 1 {
			w = 1
		}
		m.EmotionalWeight = w
	}

	// Extract chain-of-thought traces from content and store in metadata
	content, cotTrace := extractChainOfThought(m.Content)
	if cotTrace != "" {
		m.Content = content
		if req.Metadata == nil {
			req.Metadata = make(map[string]any)
		}
		req.Metadata["chainOfThought"] = cotTrace
	}

	if req.Metadata != nil {
		metaJSON, _ := json.Marshal(req.Metadata)
		m.Metadata = metaJSON
	}

	// 2. LLM Compression — extract facts, concepts, narrative, importance.
	// Enriches metadata and tags; may fill Summary if empty.
	if s.compressor != nil {
		if compressed, err := s.compressor.Compress(ctx, m.Content); err == nil && compressed != nil {
			s.compressor.EnrichMemory(m, compressed)
		} else if err != nil {
			slog.Warn("memory compression failed, continuing without enrichment", "error", err)
		}
	}

	// Compute SimHash for deduplication
	m.SimHash = SimHashToHex(ComputeSimHash(m.Content))

	// Check for near-duplicates via SimHash
	if existingHashes, err := s.memoryRepo.FindSimHashes(ctx); err == nil && len(existingHashes) > 0 {
		newHash := SimHashFromHex(m.SimHash)
		for existingID, existingHex := range existingHashes {
			existingHash := SimHashFromHex(existingHex)
			if SimHashHammingDistance(newHash, existingHash) <= 3 {
				// Near-duplicate detected: boost existing memory instead of creating new
				slog.Info("near-duplicate detected via SimHash", "existingId", existingID, "distance", SimHashHammingDistance(newHash, existingHash))
				go func() {
					bgCtx := tenant.WithTenant(context.Background(), tenant.FromContext(ctx))
					if err := s.memoryRepo.BoostAccessCount(bgCtx, existingID, 5); err != nil {
						slog.Warn("failed to boost access count", "memoryId", existingID, "error", err)
					}
				}()
				existing, err := s.memoryRepo.FindByID(ctx, existingID)
				if err == nil {
					return existing, nil
				}
			}
		}
	}

	// Auto-analyze importance and category using LLM
	if s.autoImportance && s.openRouter != nil && (m.Category == "" || m.Importance == "") {
		analysis, err := s.openRouter.AnalyzeImportance(ctx, m.Content)
		if err != nil {
			slog.Warn("failed to auto-analyze importance", "error", err)
		} else {
			if m.Category == "" {
				m.Category = domain.MemoryCategory(analysis.Category)
			}
			if m.Importance == "" {
				m.Importance = domain.ImportanceLevel(analysis.Importance)
			}
			if m.Summary == "" {
				m.Summary = analysis.Summary
			}
		}
	}

	// Generate embedding
	if s.embeddingService != nil {
		m.Embedding = s.embeddingService.Embed(m.Content)
	}

	// Set defaults
	if m.Category == "" {
		m.Category = domain.CategoryKnowledge
	}
	if m.Importance == "" {
		m.Importance = domain.ImportanceMinor
	}
	// Auto-classify memory type if not provided
	if m.MemoryType == "" {
		classifiedType, classifyConfidence := ClassifyMemoryType(m.Content, m.Tags, m.Category)
		m.MemoryType = classifiedType
		if req.Metadata == nil {
			req.Metadata = make(map[string]any)
		}
		req.Metadata["classifiedType"] = string(classifiedType)
		req.Metadata["classifyConfidence"] = classifyConfidence
		metaJSON, _ := json.Marshal(req.Metadata)
		m.Metadata = metaJSON
	}

	// Set decay rate based on memory type
	m.DecayRate = GetDecayRate(m.MemoryType)

	// Set temporal fields from request
	if req.ValidFrom != nil {
		m.ValidFrom = req.ValidFrom
	}
	if req.ValidTo != nil {
		m.ValidTo = req.ValidTo
	}

	// Provenance: honor a valid value from the request; otherwise default
	// to EXPLICIT, since a direct POST /v1/memories is the caller stating
	// the memory outright. Invalid values fall back to EXPLICIT too rather
	// than persisting garbage.
	if req.Provenance.IsValid() {
		m.Provenance = req.Provenance
	} else {
		m.Provenance = domain.ProvenanceExplicit
	}

	// Check for temporal supersession: if a similar memory with same subject exists, supersede it
	if m.SimHash != "" {
		if existingHashes, err := s.memoryRepo.FindSimHashes(ctx); err == nil {
			newHash := SimHashFromHex(m.SimHash)
			for existingID, existingHex := range existingHashes {
				existingHash := SimHashFromHex(existingHex)
				dist := SimHashHammingDistance(newHash, existingHash)
				// Near-match (4-8 distance) with same category = candidate for supersession
				if dist > 3 && dist <= 8 && m.Category != "" {
					existing, err := s.memoryRepo.FindByID(ctx, existingID)
					if err == nil && existing.Category == m.Category && existing.SupersededBy == "" {
						// Supersede the old memory + propagate staleness through graph.
						go func(oldID, newID, tid string) {
							bgCtx := tenant.WithTenant(context.Background(), tid)
							if err := s.memoryRepo.SupersedeMemory(bgCtx, oldID, newID); err != nil {
								slog.Warn("supersede failed", "oldId", oldID, "error", err)
								return
							}
							if s.stalenessSvc != nil {
								if _, err := s.stalenessSvc.PropagateFromSupersession(bgCtx, oldID, newID); err != nil {
									slog.Warn("staleness propagation failed", "oldId", oldID, "error", err)
								}
							}
						}(existingID, m.ID, tenant.FromContext(ctx))
						break
					}
				}
			}
		}
	}

	if err := s.memoryRepo.Create(ctx, m); err != nil {
		return nil, err
	}

	// Create initial version
	if s.versionRepo != nil {
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), m.TenantID)
			if err := s.versionRepo.CreateFromMemory(bgCtx, m, "create", "initial creation", m.CreatedBy); err != nil {
				slog.Warn("failed to create initial version", "error", err, "memoryId", m.ID)
			}
		}()
	}

	// Audit log
	if s.auditService != nil {
		go s.auditService.LogMemoryCreated(tenant.WithTenant(context.Background(), m.TenantID), m)
	}

	// 3. Triplet extraction — async, non-blocking. Results stored in metadata.
	// (Triplet persistence to a dedicated table/collection is a future enhancement.)
	if s.tripletSvc != nil {
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), m.TenantID)
			triplets, err := s.tripletSvc.ExtractAndBuild(bgCtx, m.ID, m.Content)
			if err != nil {
				slog.Warn("triplet extraction failed", "memoryId", m.ID, "error", err)
				return
			}
			if len(triplets) == 0 {
				return
			}
			// Merge triplet summaries into metadata for discoverability.
			meta := make(map[string]any)
			if len(m.Metadata) > 0 {
				_ = json.Unmarshal(m.Metadata, &meta)
			}
			tripletSummaries := make([]string, 0, len(triplets))
			for _, t := range triplets {
				tripletSummaries = append(tripletSummaries, t.Text)
			}
			meta["triplets"] = tripletSummaries
			meta["tripletCount"] = len(triplets)
			if raw, err := json.Marshal(meta); err == nil {
				m.Metadata = raw
				_ = s.memoryRepo.Update(bgCtx, m)
			}
		}()
	}

	// 4. Event extraction — async, non-blocking. The LLM looks for structured
	// occurrences inside the content and persists them with source_memory_id.
	if s.eventSvc != nil {
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), m.TenantID)
			if _, err := s.eventSvc.ExtractFromText(bgCtx, m.Content, m.ID); err != nil {
				slog.Warn("event extraction failed", "memoryId", m.ID, "error", err)
			}
		}()
	}

	return m, nil
}

// GetMemory retrieves a memory by ID and tracks access.
func (s *MemoryService) GetMemory(ctx context.Context, id string) (*domain.Memory, error) {
	m, err := s.memoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, domain.NewNotFoundError("memory not found: " + id)
	}

	// Track access asynchronously
	go func() {
		bgCtx := tenant.WithTenant(context.Background(), m.TenantID)
		if err := s.memoryRepo.IncrementAccessCount(bgCtx, id); err != nil {
			slog.Warn("failed to increment access count", "memoryId", id, "error", err)
		}
	}()

	return m, nil
}

// ListMemories returns paginated memories.
func (s *MemoryService) ListMemories(ctx context.Context, page, size int) (*dto.MemoryListResponse, error) {
	if size <= 0 {
		size = 20
	}
	if page < 0 {
		page = 0
	}

	memories, total, err := s.memoryRepo.List(ctx, page, size)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / size
	if int(total)%size > 0 {
		totalPages++
	}

	resp := &dto.MemoryListResponse{
		Memories:      make([]dto.MemoryResponse, 0, len(memories)),
		Page:          page,
		Size:          size,
		TotalElements: total,
		TotalPages:    totalPages,
		HasNext:       page < totalPages-1,
		HasPrevious:   page > 0,
	}

	for _, m := range memories {
		resp.Memories = append(resp.Memories, memoryToResponse(m))
	}

	return resp, nil
}

// UpdateMemory updates a memory with versioning.
func (s *MemoryService) UpdateMemory(ctx context.Context, id string, req dto.UpdateMemoryRequest) (*domain.Memory, error) {
	m, err := s.memoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Archive current version before updating
	if s.versionRepo != nil {
		go func() {
			bgCtx := tenant.WithTenant(context.Background(), m.TenantID)
			if err := s.versionRepo.CreateFromMemory(bgCtx, m, "update", req.ChangeReason, ""); err != nil {
				slog.Warn("failed to create version", "error", err)
			}
		}()
	}

	// Apply updates
	if req.Content != "" {
		m.Content = req.Content
	}
	if req.Summary != "" {
		m.Summary = req.Summary
	}
	if req.Category != "" {
		m.Category = req.Category
	}
	if req.Importance != "" {
		m.Importance = req.Importance
	}
	if req.Tags != nil {
		m.Tags = req.Tags
	}
	if req.Metadata != nil {
		metaJSON, _ := json.Marshal(req.Metadata)
		m.Metadata = metaJSON
	}
	if req.CodeExample != "" {
		m.CodeExample = req.CodeExample
	}
	if req.ProgrammingLanguage != "" {
		m.ProgrammingLanguage = req.ProgrammingLanguage
	}

	m.Version++

	// Regenerate embedding if content changed
	if req.Content != "" && s.embeddingService != nil {
		m.Embedding = s.embeddingService.Embed(m.Content)
	}

	if err := s.memoryRepo.Update(ctx, m); err != nil {
		return nil, err
	}

	// Audit
	if s.auditService != nil {
		go s.auditService.LogMemoryUpdated(tenant.WithTenant(context.Background(), m.TenantID), m)
	}

	return m, nil
}

// DeleteMemory soft-deletes a memory by setting deleted_at timestamp.
func (s *MemoryService) DeleteMemory(ctx context.Context, id string) error {
	if err := s.memoryRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Audit
	if s.auditService != nil {
		go s.auditService.LogMemoryDeleted(tenant.WithTenant(context.Background(), tenant.FromContext(ctx)), id)
	}

	return nil
}

// SearchMemories searches memories by text query, using vector search when available.
// Results are re-ranked using composite hybrid scoring with explainable traces.
func (s *MemoryService) SearchMemories(ctx context.Context, req dto.SearchRequest) (*dto.SearchResponse, error) {
	start := time.Now()
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	// Query expansion: generate reformulations to broaden recall.
	// Primary query is always first; reformulations augment (deduplicated).
	expandedQueries := []string{req.Query}
	if s.queryExpander != nil {
		if expansion := s.queryExpander.Expand(ctx, req.Query); expansion != nil {
			for _, r := range expansion.Reformulations {
				if r != "" && r != req.Query {
					expandedQueries = append(expandedQueries, r)
				}
			}
		}
	}

	queryTokens := TokenizeQuery(req.Query)
	scoredByID := make(map[string]scoredMemory)

	addScored := func(m *domain.Memory, sim float64) {
		existing, found := scoredByID[m.ID]
		newTrace := ComputeHybridScore(m, sim, queryTokens, -1, req.Tags, DefaultScoringWeights)
		if !found || newTrace.FinalScore > existing.trace.FinalScore {
			scoredByID[m.ID] = scoredMemory{memory: *m, trace: newTrace}
		}
	}

	for _, q := range expandedQueries {
		// Vector search per query (when available)
		if s.memoryGraphRepo != nil && s.embeddingService != nil && s.embeddingService.HasAPI() {
			embedding := s.embeddingService.Embed(q)
			ids, scores, err := s.memoryGraphRepo.VectorSearch(ctx, embedding, limit*2, tenant.FromContext(ctx))
			if err == nil {
				for i, id := range ids {
					m, err := s.memoryRepo.FindByID(ctx, id)
					if err != nil || isInactiveMemory(m, time.Now()) {
						continue
					}
					addScored(m, scores[i])
				}
			}
		}
	}

	// Full-text search as fallback/supplement (primary query only to avoid duplicate work)
	if len(scoredByID) < limit {
		textResults, err := s.memoryRepo.FullTextSearch(ctx, req.Query, limit)
		if err == nil {
			for i := range textResults {
				m := &textResults[i]
				if _, exists := scoredByID[m.ID]; exists {
					continue
				}
				if isInactiveMemory(m, time.Now()) {
					continue
				}
				addScored(m, 0.3)
			}
		}
	}

	// Collect into slice for sorting
	scoredResults := make([]scoredMemory, 0, len(scoredByID))
	for _, sr := range scoredByID {
		// Feedback blending: apply before sort so top-N reflects learned quality.
		if s.feedbackSvc != nil {
			s.feedbackSvc.ApplyFeedbackToTrace(&sr.trace, &sr.memory)
		}
		scoredResults = append(scoredResults, sr)
	}

	// Sort by hybrid score descending
	sortScoredMemories(scoredResults)

	// Trim to limit
	if len(scoredResults) > limit {
		scoredResults = scoredResults[:limit]
	}

	result := make([]dto.MemoryResponse, 0, len(scoredResults))
	for _, sr := range scoredResults {
		resp := memoryToResponse(sr.memory)
		resp.RelevanceScore = sr.trace.FinalScore
		result = append(result, resp)
	}

	return &dto.SearchResponse{
		Results:      result,
		Total:        len(result),
		SearchTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

type scoredMemory struct {
	memory domain.Memory
	trace  ScoreTrace
}

func sortScoredMemories(results []scoredMemory) {
	// Simple insertion sort (small lists)
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].trace.FinalScore < key.trace.FinalScore {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}

// GetByCategory returns memories filtered by category.
func (s *MemoryService) GetByCategory(ctx context.Context, category domain.MemoryCategory) ([]dto.MemoryResponse, error) {
	memories, err := s.memoryRepo.FindByCategory(ctx, category)
	if err != nil {
		return nil, err
	}
	result := make([]dto.MemoryResponse, 0, len(memories))
	for _, m := range memories {
		result = append(result, memoryToResponse(m))
	}
	return result, nil
}

// GetByImportance returns memories filtered by importance.
func (s *MemoryService) GetByImportance(ctx context.Context, importance domain.ImportanceLevel) ([]dto.MemoryResponse, error) {
	memories, err := s.memoryRepo.FindByImportance(ctx, importance)
	if err != nil {
		return nil, err
	}
	result := make([]dto.MemoryResponse, 0, len(memories))
	for _, m := range memories {
		result = append(result, memoryToResponse(m))
	}
	return result, nil
}

// RecordFeedback records helpful/not helpful feedback.
func (s *MemoryService) RecordFeedback(ctx context.Context, id string, helpful bool) error {
	return s.memoryRepo.RecordFeedback(ctx, id, helpful)
}

// GetVersionHistory returns the version history for a memory.
func (s *MemoryService) GetVersionHistory(ctx context.Context, memoryID string) ([]domain.MemoryVersion, error) {
	if s.versionRepo == nil {
		return nil, nil
	}
	return s.versionRepo.FindByMemoryID(ctx, memoryID)
}

func memoryToResponse(m domain.Memory) dto.MemoryResponse {
	var metadata map[string]any
	if m.Metadata != nil {
		json.Unmarshal(m.Metadata, &metadata)
	}

	return dto.MemoryResponse{
		ID:                  m.ID,
		Content:             m.Content,
		Summary:             m.Summary,
		Category:            m.Category,
		Importance:          m.Importance,
		ValidationStatus:    m.ValidationStatus,
		Metadata:            metadata,
		Tags:                m.Tags,
		SourceType:          m.SourceType,
		SourceReference:     m.SourceReference,
		CreatedBy:           m.CreatedBy,
		TenantID:            m.TenantID,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		LastAccessedAt:      m.LastAccessedAt,
		Version:             m.Version,
		AccessCount:         m.AccessCount,
		InjectionCount:      m.InjectionCount,
		HelpfulCount:        m.HelpfulCount,
		NotHelpfulCount:     m.NotHelpfulCount,
		HelpfulnessRate:     m.HelpfulnessRate(),
		RelevanceScore:      m.RelevanceScore(),
		CodeExample:         m.CodeExample,
		ProgrammingLanguage: m.ProgrammingLanguage,
		MemoryType:          m.MemoryType,
		EmotionalWeight:     m.EmotionalWeight,
		SimHash:             m.SimHash,
		ValidFrom:           m.ValidFrom,
		ValidTo:             m.ValidTo,
		DecayRate:           m.DecayRate,
		SupersededBy:        m.SupersededBy,
		DecayedRelevance:    ComputeDecayedRelevance(&m, time.Now()),
	}
}

var cotPattern = regexp.MustCompile(`(?s)<THOUGHT>(.*?)</THOUGHT>`)

// extractChainOfThought extracts <THOUGHT>...</THOUGHT> blocks from content.
// Returns cleaned content and concatenated thought traces.
func extractChainOfThought(content string) (string, string) {
	matches := cotPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, ""
	}

	var thoughts []string
	for _, m := range matches {
		thoughts = append(thoughts, strings.TrimSpace(m[1]))
	}

	cleaned := cotPattern.ReplaceAllString(content, "")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned, strings.Join(thoughts, "\n---\n")
}
