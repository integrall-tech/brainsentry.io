package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	scheduler      *TaskScheduler                // durable queue for heavy extractions (optional)
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

// WithTaskScheduler routes the heavy (LLM-bound) extractions — triplets and
// events — through the durable Redis-Streams queue instead of a fire-and-forget
// goroutine, so a process crash mid-extraction is retried/recovered rather than
// silently lost. When nil (no Redis), CreateMemory keeps the original goroutine
// behavior. The composition root must also RegisterHandler for the extraction
// task types, or the work would be dropped.
func (s *MemoryService) WithTaskScheduler(ts *TaskScheduler) *MemoryService {
	s.scheduler = ts
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
	FindDedupCandidates(ctx context.Context, f postgres.DedupCandidateFilter) (map[string]string, error)
	FindBySourceReference(ctx context.Context, sourceReference string) (*domain.Memory, error)
	BoostAccessCount(ctx context.Context, id string, boost int) error
	SupersedeMemory(ctx context.Context, oldID, newID string) error
	FindByExactFilter(ctx context.Context, f postgres.ExactFilter) ([]domain.Memory, error)
	BatchExpire(ctx context.Context, ids []string, sourceReference, reason string) (*postgres.BatchExpireResult, error)
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
	// 0. Idempotency by origin. The Core's outbox is at-least-once: it retries
	// when it cannot be sure the previous call landed. One domain event must
	// produce one memory, so the retry returns the SAME id instead of a second
	// fact.
	//
	// Runs first, before stripping/compression/embedding, because a retry
	// should cost a single indexed lookup — not another LLM round-trip for a
	// memory that already exists.
	//
	// Until now the retry was absorbed by the content dedup, which this change
	// correctly restricts to a tag scope; without this guard, restricting the
	// dedup would have turned every outbox retry into a duplicate fact.
	if req.SourceReference != "" {
		if existing, err := s.memoryRepo.FindBySourceReference(ctx, req.SourceReference); err == nil && existing != nil {
			slog.Info("memory already exists for this origin, returning it",
				"sourceReference", req.SourceReference, "memoryId", existing.ID)
			return existing, nil
		} else if err != nil {
			// A lookup failure must not silently become "create a duplicate".
			return nil, fmt.Errorf("checking existing memory for source reference: %w", err)
		}
	}

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

	// Near-duplicate check, now SCOPED. Two guards, in order:
	//
	// 1. A memory with a sourceReference comes from a distinct domain event,
	//    so it is a distinct fact by construction — textual similarity gets no
	//    vote. Idempotency for that case is handled above, by origin.
	//
	// 2. Otherwise, compare only against memories sharing the tags the caller
	//    declared. The old code compared against the whole tenant, and the
	//    VendaX write pattern (one templated sentence per customer) makes
	//    byte-identical content across customers routine: the second POST fell
	//    at distance 0, created nothing, and returned a memory tagged for a
	//    DIFFERENT customer. The Core believed it had written; the customer
	//    never got the fact; nothing recorded the suppression.
	//
	// req.Tags, not m.Tags: the compressor appends LLM-derived tags just above,
	// and those are not deterministic — scoping on them would make the same
	// fact fail to deduplicate against itself.
	existingHashes := map[string]string{}
	if m.SourceReference == "" {
		if found, err := s.memoryRepo.FindDedupCandidates(ctx, postgres.DedupCandidateFilter{
			SimHash:   m.SimHash,
			ScopeTags: req.Tags,
		}); err == nil {
			existingHashes = found
		}
	}
	if len(existingHashes) > 0 {
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

	// 3. Triplet extraction — heavy (LLM). Routed through the durable scheduler
	// when available, else a detached goroutine. Results stored in metadata.
	// (Triplet persistence to a dedicated table/collection is a future enhancement.)
	if s.tripletSvc != nil {
		s.dispatchExtraction(ctx, TaskTripletExtraction, m.TenantID, m.ID, m.Content)
	}

	// 4. Event extraction — heavy (LLM). The model looks for structured
	// occurrences inside the content and persists them with source_memory_id.
	if s.eventSvc != nil {
		s.dispatchExtraction(ctx, TaskEventExtraction, m.TenantID, m.ID, m.Content)
	}

	return m, nil
}

// dispatchExtraction enqueues a heavy extraction on the durable task scheduler,
// falling back to a detached goroutine when no scheduler is configured or the
// enqueue fails — so the work is never silently dropped on the create path.
func (s *MemoryService) dispatchExtraction(ctx context.Context, taskType TaskType, tenantID, memoryID, content string) {
	if s.scheduler != nil {
		payload := extractionPayload{MemoryID: memoryID, Content: content}
		if _, err := s.scheduler.Submit(ctx, taskType, tenantID, "", PriorityNormal, payload); err == nil {
			return
		}
		slog.Warn("scheduler submit failed; running extraction inline", "type", taskType, "memoryId", memoryID)
	}
	go func() {
		bgCtx := tenant.WithTenant(context.Background(), tenantID)
		if err := s.runExtraction(bgCtx, taskType, memoryID, content); err != nil {
			slog.Warn("extraction failed", "type", taskType, "memoryId", memoryID, "error", err)
		}
	}()
}

// runExtraction executes a single extraction by type. It is the shared body for
// both the goroutine fallback above and the scheduler handler (extraction_tasks.go),
// so the two paths can never diverge.
func (s *MemoryService) runExtraction(ctx context.Context, taskType TaskType, memoryID, content string) error {
	switch taskType {
	case TaskTripletExtraction:
		return s.runTripletExtraction(ctx, memoryID, content)
	case TaskEventExtraction:
		return s.runEventExtraction(ctx, memoryID, content)
	default:
		return fmt.Errorf("unknown extraction task type: %s", taskType)
	}
}

// runTripletExtraction extracts S-P-O triplets and merges a summary into the
// memory's metadata. It reloads the memory by ID (rather than capturing the
// create-path pointer) so the durable handler is self-contained and there is no
// data race against the value returned to the caller.
func (s *MemoryService) runTripletExtraction(ctx context.Context, memoryID, content string) error {
	if s.tripletSvc == nil {
		return nil
	}
	triplets, err := s.tripletSvc.ExtractAndBuild(ctx, memoryID, content)
	if err != nil {
		return fmt.Errorf("triplet extraction: %w", err)
	}
	if len(triplets) == 0 {
		return nil
	}

	m, err := s.memoryRepo.FindByID(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("reload memory for triplet metadata: %w", err)
	}

	meta := make(map[string]any)
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &meta)
	}
	summaries := make([]string, 0, len(triplets))
	for _, t := range triplets {
		summaries = append(summaries, t.Text)
	}
	meta["triplets"] = summaries
	meta["tripletCount"] = len(triplets)

	raw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal triplet metadata: %w", err)
	}
	m.Metadata = raw
	if err := s.memoryRepo.Update(ctx, m); err != nil {
		return fmt.Errorf("persist triplet metadata: %w", err)
	}
	return nil
}

// runEventExtraction extracts structured events from the content and persists
// them with source_memory_id.
func (s *MemoryService) runEventExtraction(ctx context.Context, memoryID, content string) error {
	if s.eventSvc == nil {
		return nil
	}
	if _, err := s.eventSvc.ExtractFromText(ctx, content, memoryID); err != nil {
		return fmt.Errorf("event extraction: %w", err)
	}
	return nil
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

	// Deterministic route (RFC-014 fatia 1). Taken FIRST, before query
	// expansion, embedding or scoring — those are not merely wasted here,
	// they are wrong: an exact lookup that ranks by similarity can return a
	// different memory than the one asked for.
	if req.IsExact() {
		return s.searchExact(ctx, req, limit, start)
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

// searchExact answers a lookup by identity: source reference and/or metadata
// pairs, newest first.
//
// Deliberately does NOT touch the embedding service, the query expander or
// the hybrid scorer. Beyond the wasted latency and embedding spend, ranking a
// known key by similarity can put a *different* memory first — the audit
// would then compare the wrong fact against the source and revoke the wrong
// row.
//
// It also does not increment access counts: an audit sweep reading every fact
// would otherwise inflate the very signal that access-based ranking uses.
func (s *MemoryService) searchExact(ctx context.Context, req dto.SearchRequest, limit int, start time.Time) (*dto.SearchResponse, error) {
	memories, err := s.memoryRepo.FindByExactFilter(ctx, postgres.ExactFilter{
		SourceReference: req.SourceReference,
		Metadata:        req.Metadata,
		Limit:           limit,
	})
	if err != nil {
		return nil, err
	}

	results := make([]dto.MemoryResponse, 0, len(memories))
	for _, m := range memories {
		results = append(results, memoryToResponse(m))
	}

	return &dto.SearchResponse{
		Results:      results,
		Total:        len(results),
		SearchTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

// BatchExpireMemories closes the validity window of many memories in one
// transaction (RFC-014 fatia 1). Used by the audit routine, which revokes in
// bulk when a source event is reverted.
func (s *MemoryService) BatchExpireMemories(ctx context.Context, req dto.BatchExpireRequest) (*postgres.BatchExpireResult, error) {
	if len(req.Ids) == 0 && req.SourceReference == "" {
		return nil, fmt.Errorf("ids or sourceReference is required")
	}
	if req.Reason == "" {
		// A bulk revocation with no reason is unauditable later — which
		// defeats the point of a routine whose output is a report.
		return nil, fmt.Errorf("reason is required")
	}
	return s.memoryRepo.BatchExpire(ctx, req.Ids, req.SourceReference, req.Reason)
}
