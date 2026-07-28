package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// --- extractChainOfThought tests ---

func TestCOTExtraction_NoThought(t *testing.T) {
	content, cot := extractChainOfThought("normal content without thought tags")
	if cot != "" {
		t.Errorf("expected empty COT, got %q", cot)
	}
	if content != "normal content without thought tags" {
		t.Errorf("expected unchanged content, got %q", content)
	}
}

func TestCOTExtraction_SingleThought(t *testing.T) {
	input := "before <THOUGHT>my reasoning</THOUGHT> after"
	content, cot := extractChainOfThought(input)
	if cot != "my reasoning" {
		t.Errorf("expected 'my reasoning', got %q", cot)
	}
	if content != "before  after" {
		t.Errorf("expected 'before  after', got %q", content)
	}
}

func TestCOTExtraction_MultipleThoughts(t *testing.T) {
	input := "<THOUGHT>first</THOUGHT> text <THOUGHT>second</THOUGHT>"
	content, cot := extractChainOfThought(input)
	if cot != "first\n---\nsecond" {
		t.Errorf("expected 'first\\n---\\nsecond', got %q", cot)
	}
	if content != "text" {
		t.Errorf("expected 'text', got %q", content)
	}
}

// --- EmotionalWeight clamping (tested via CreateMemory logic inlined) ---

func TestEmotionalWeightClamping(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{"above max", 1.5, 1.0},
		{"below min", -2.0, -1.0},
		{"within range", 0.5, 0.5},
		{"at max", 1.0, 1.0},
		{"at min", -1.0, -1.0},
		{"zero", 0.0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.input
			if w < -1 {
				w = -1
			}
			if w > 1 {
				w = 1
			}
			if w != tt.want {
				t.Errorf("clamp(%f) = %f, want %f", tt.input, w, tt.want)
			}
		})
	}
}

// --- RelevanceScore formula tests ---

func TestRelevanceScore_Formula(t *testing.T) {
	tests := []struct {
		name           string
		accessCount    int
		injectionCount int
		helpful        int
		notHelpful     int
		expected       float64
	}{
		{
			name:           "standard counts",
			accessCount:    10,
			injectionCount: 5,
			helpful:        3,
			notHelpful:     1,
			expected:       10*0.3 + 5*0.4 + 0.75*0.3, // 3 + 2 + 0.225 = 5.225
		},
		{
			name:           "zero everything",
			accessCount:    0,
			injectionCount: 0,
			helpful:        0,
			notHelpful:     0,
			expected:       0,
		},
		{
			name:           "all helpful",
			accessCount:    1,
			injectionCount: 1,
			helpful:        10,
			notHelpful:     0,
			expected:       1*0.3 + 1*0.4 + 1.0*0.3, // 0.3 + 0.4 + 0.3 = 1.0
		},
		{
			name:           "all not helpful",
			accessCount:    5,
			injectionCount: 0,
			helpful:        0,
			notHelpful:     10,
			expected:       5*0.3 + 0 + 0, // 1.5
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &domain.Memory{
				AccessCount:     tt.accessCount,
				InjectionCount:  tt.injectionCount,
				HelpfulCount:    tt.helpful,
				NotHelpfulCount: tt.notHelpful,
			}
			score := m.RelevanceScore()
			if math.Abs(score-tt.expected) > 0.001 {
				t.Errorf("RelevanceScore() = %f, want %f", score, tt.expected)
			}
		})
	}
}

// --- HelpfulnessRate tests ---

func TestHelpfulnessRate_ZeroDenominator(t *testing.T) {
	m := &domain.Memory{HelpfulCount: 0, NotHelpfulCount: 0}
	if m.HelpfulnessRate() != 0 {
		t.Errorf("expected 0 for zero counts, got %f", m.HelpfulnessRate())
	}
}

func TestHelpfulnessRate_AllHelpful(t *testing.T) {
	m := &domain.Memory{HelpfulCount: 10, NotHelpfulCount: 0}
	if m.HelpfulnessRate() != 1.0 {
		t.Errorf("expected 1.0 for all helpful, got %f", m.HelpfulnessRate())
	}
}

func TestHelpfulnessRate_FiftyFifty(t *testing.T) {
	m := &domain.Memory{HelpfulCount: 5, NotHelpfulCount: 5}
	if m.HelpfulnessRate() != 0.5 {
		t.Errorf("expected 0.5 for equal counts, got %f", m.HelpfulnessRate())
	}
}

// --- DecayRate from MemoryType ---

func TestDecayRateFromMemoryType(t *testing.T) {
	tests := []struct {
		memType domain.MemoryType
		rate    float64
	}{
		{domain.MemoryTypePersonality, 0.001},
		{domain.MemoryTypeThread, 0.050},
		{domain.MemoryTypeTask, 0.030},
		{domain.MemoryTypeSemantic, 0.005},
		{domain.MemoryTypeProcedural, 0.005},
		{domain.MemoryTypeEpisodic, 0.015},
		{domain.MemoryTypeAssociative, 0.010},
	}
	for _, tt := range tests {
		t.Run(string(tt.memType), func(t *testing.T) {
			rate := GetDecayRate(tt.memType)
			if rate != tt.rate {
				t.Errorf("GetDecayRate(%s) = %f, want %f", tt.memType, rate, tt.rate)
			}
		})
	}
}

// --- sortScoredMemories tests ---

func TestSortScoredMemories_Descending(t *testing.T) {
	results := []scoredMemory{
		{memory: domain.Memory{ID: "low"}, trace: ScoreTrace{FinalScore: 0.3}},
		{memory: domain.Memory{ID: "high"}, trace: ScoreTrace{FinalScore: 0.9}},
		{memory: domain.Memory{ID: "mid"}, trace: ScoreTrace{FinalScore: 0.6}},
	}
	sortScoredMemories(results)
	if results[0].memory.ID != "high" {
		t.Errorf("expected 'high' first, got %s", results[0].memory.ID)
	}
	if results[2].memory.ID != "low" {
		t.Errorf("expected 'low' last, got %s", results[2].memory.ID)
	}
}

func TestSortScoredMemories_SingleElement(t *testing.T) {
	results := []scoredMemory{
		{memory: domain.Memory{ID: "only"}, trace: ScoreTrace{FinalScore: 0.5}},
	}
	sortScoredMemories(results)
	if results[0].memory.ID != "only" {
		t.Errorf("expected 'only', got %s", results[0].memory.ID)
	}
}

func TestSortScoredMemories_Empty(t *testing.T) {
	var results []scoredMemory
	sortScoredMemories(results) // should not panic
}

func TestSortScoredMemories_AllSameScore(t *testing.T) {
	results := []scoredMemory{
		{memory: domain.Memory{ID: "a"}, trace: ScoreTrace{FinalScore: 0.5}},
		{memory: domain.Memory{ID: "b"}, trace: ScoreTrace{FinalScore: 0.5}},
		{memory: domain.Memory{ID: "c"}, trace: ScoreTrace{FinalScore: 0.5}},
	}
	sortScoredMemories(results) // should not panic
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}
}

// --- NewMemoryService tests ---

func TestNewMemoryService_NilDeps(t *testing.T) {
	svc := NewMemoryService(nil, nil, nil, nil, nil, nil, true)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.piiService == nil {
		t.Error("piiService should be auto-initialized")
	}
	if !svc.autoImportance {
		t.Error("autoImportance should be true")
	}
}

func TestNewMemoryService_AutoImportanceFalse(t *testing.T) {
	svc := NewMemoryService(nil, nil, nil, nil, nil, nil, false)
	if svc.autoImportance {
		t.Error("autoImportance should be false")
	}
}

// --- memoryToResponse tests ---

func TestMemoryToResponse_PreservesFields(t *testing.T) {
	m := domain.Memory{
		ID:         "test-id",
		Content:    "test content",
		Summary:    "test summary",
		Category:   domain.CategoryKnowledge,
		Importance: domain.ImportanceCritical,
		Tags:       []string{"go", "test"},
		MemoryType: domain.MemoryTypeSemantic,
		Version:    3,
		CreatedAt:  time.Now(),
	}
	resp := memoryToResponse(m)
	if resp.ID != m.ID {
		t.Errorf("ID mismatch: %s vs %s", resp.ID, m.ID)
	}
	if resp.Category != m.Category {
		t.Errorf("Category mismatch: %s vs %s", resp.Category, m.Category)
	}
	if resp.Version != m.Version {
		t.Errorf("Version mismatch: %d vs %d", resp.Version, m.Version)
	}
}

func TestMemoryToResponse_IncludesComputedFields(t *testing.T) {
	m := domain.Memory{
		AccessCount:     10,
		InjectionCount:  5,
		HelpfulCount:    3,
		NotHelpfulCount: 1,
		MemoryType:      domain.MemoryTypeSemantic,
		CreatedAt:       time.Now(),
	}
	resp := memoryToResponse(m)
	if resp.RelevanceScore <= 0 {
		t.Errorf("expected positive relevance score, got %f", resp.RelevanceScore)
	}
	if resp.HelpfulnessRate != 0.75 {
		t.Errorf("expected 0.75 helpfulness rate, got %f", resp.HelpfulnessRate)
	}
}

type fakeMemoryRepository struct {
	byID              map[string]*domain.Memory
	fullTextResults   []domain.Memory
	fullTextQueries   []string
	fullTextTagScopes [][]string

	exactCalls   []postgres.ExactFilter
	exactResults []domain.Memory
	exactErr     error
	expireCalls  []batchExpireCall
	expireErr    error

	// Dedup bookkeeping. dedupFilters records every filter the service asked
	// for — the service's side of the contract is WHICH scope it requests.
	dedupFilters []postgres.DedupCandidateFilter
	seq          int
	bySourceRef  map[string]*domain.Memory
}

// FindDedupCandidates mimics the repository's scoping so a service test can
// assert end-to-end behaviour: a memory only qualifies when it carries every
// tag the caller declared. The SQL that implements this for real is exercised
// against a live schema, not here.
func (f *fakeMemoryRepository) FindDedupCandidates(_ context.Context, filter postgres.DedupCandidateFilter) (map[string]string, error) {
	f.dedupFilters = append(f.dedupFilters, filter)

	out := map[string]string{}
	for id, m := range f.byID {
		if m.SimHash == "" {
			continue
		}
		if !hasAllTags(m.Tags, filter.ScopeTags) {
			continue
		}
		out[id] = m.SimHash
	}
	return out, nil
}

func hasAllTags(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (f *fakeMemoryRepository) FindBySourceReference(_ context.Context, sourceReference string) (*domain.Memory, error) {
	if sourceReference == "" || f.bySourceRef == nil {
		return nil, nil
	}
	return f.bySourceRef[sourceReference], nil
}

func (f *fakeMemoryRepository) Create(_ context.Context, m *domain.Memory) error {
	if f.byID == nil {
		f.byID = make(map[string]*domain.Memory)
	}
	// The real repository assigns an id here. Without it every created memory
	// lands under the key "" and two distinct memories look like one — which
	// silently weakens any test that counts rows or compares ids.
	if m.ID == "" {
		f.seq++
		m.ID = fmt.Sprintf("mem-%d", f.seq)
	}
	copyMemory := *m
	f.byID[m.ID] = &copyMemory
	return nil
}

func (f *fakeMemoryRepository) FindByID(_ context.Context, id string) (*domain.Memory, error) {
	if f.byID != nil {
		if m, ok := f.byID[id]; ok {
			copyMemory := *m
			return &copyMemory, nil
		}
	}
	return nil, errors.New("memory not found")
}

func (f *fakeMemoryRepository) List(_ context.Context, _ int, _ int) ([]domain.Memory, int64, error) {
	return nil, 0, nil
}

func (f *fakeMemoryRepository) Update(_ context.Context, m *domain.Memory) error {
	if f.byID == nil {
		f.byID = make(map[string]*domain.Memory)
	}
	copyMemory := *m
	f.byID[m.ID] = &copyMemory
	return nil
}

func (f *fakeMemoryRepository) Delete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

func (f *fakeMemoryRepository) FindByCategory(_ context.Context, _ domain.MemoryCategory) ([]domain.Memory, error) {
	return nil, nil
}

func (f *fakeMemoryRepository) FindByImportance(_ context.Context, _ domain.ImportanceLevel) ([]domain.Memory, error) {
	return nil, nil
}

func (f *fakeMemoryRepository) FullTextSearch(ctx context.Context, query string, limit int) ([]domain.Memory, error) {
	return f.FullTextSearchScoped(ctx, query, limit, nil)
}

// FullTextSearchScoped mimics the real query: the tag restriction is applied
// BEFORE the limit, because that ordering is the property under test. Applying
// it after would silently return fewer rows than exist — trading a leak for
// "this customer has no such fact", which is worse for looking like an answer.
func (f *fakeMemoryRepository) FullTextSearchScoped(_ context.Context, query string, limit int, tags []string) ([]domain.Memory, error) {
	f.fullTextQueries = append(f.fullTextQueries, query)
	f.fullTextTagScopes = append(f.fullTextTagScopes, tags)

	scoped := make([]domain.Memory, 0, len(f.fullTextResults))
	for _, m := range f.fullTextResults {
		if hasAllTags(m.Tags, tags) {
			scoped = append(scoped, m)
		}
	}
	if limit > 0 && len(scoped) > limit {
		return scoped[:limit], nil
	}
	return scoped, nil
}

func (f *fakeMemoryRepository) FindByIDsScoped(_ context.Context, ids []string, tags []string) ([]domain.Memory, error) {
	var out []domain.Memory
	for _, id := range ids {
		m, ok := f.byID[id]
		if !ok || !hasAllTags(m.Tags, tags) {
			continue
		}
		out = append(out, *m)
	}
	return out, nil
}

func (f *fakeMemoryRepository) IncrementAccessCount(_ context.Context, _ string) error {
	return nil
}

func (f *fakeMemoryRepository) RecordFeedback(_ context.Context, _ string, _ bool) error {
	return nil
}

func (f *fakeMemoryRepository) FindSimHashes(_ context.Context) (map[string]string, error) {
	return nil, nil
}

func (f *fakeMemoryRepository) BoostAccessCount(_ context.Context, _ string, _ int) error {
	return nil
}

func (f *fakeMemoryRepository) SupersedeMemory(_ context.Context, _ string, _ string) error {
	return nil
}

// exactCalls records deterministic lookups so tests can assert that the
// semantic path was not taken (and vice versa).
func (f *fakeMemoryRepository) FindByExactFilter(_ context.Context, filter postgres.ExactFilter) ([]domain.Memory, error) {
	f.exactCalls = append(f.exactCalls, filter)
	return f.exactResults, f.exactErr
}

func (f *fakeMemoryRepository) BatchExpire(_ context.Context, ids []string, sourceReference, reason string) (*postgres.BatchExpireResult, error) {
	f.expireCalls = append(f.expireCalls, batchExpireCall{IDs: ids, SourceReference: sourceReference, Reason: reason})
	if f.expireErr != nil {
		return nil, f.expireErr
	}
	return &postgres.BatchExpireResult{Expired: int64(len(ids)), IDs: ids}, nil
}

type batchExpireCall struct {
	IDs             []string
	SourceReference string
	Reason          string
}

type fakeMemoryGraphRepository struct {
	ids    []string
	scores []float64
}

func (f *fakeMemoryGraphRepository) VectorSearch(_ context.Context, _ []float32, _ int, _ string) ([]string, []float64, error) {
	return f.ids, f.scores, nil
}

type fakeEmbeddingGenerator struct {
	api bool
}

func (f fakeEmbeddingGenerator) Embed(_ string) []float32 {
	return []float32{1, 0, 0}
}

func (f fakeEmbeddingGenerator) HasAPI() bool {
	return f.api
}

func TestSearchMemories_TextFallbackFiltersInactiveAndSorts(t *testing.T) {
	now := time.Now()
	expiredAt := now.Add(-time.Hour)
	repo := &fakeMemoryRepository{
		fullTextResults: []domain.Memory{
			{
				ID:         "lower-score",
				Content:    "postgres memory",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceMinor,
				// Era []string{"other"}. Este teste é sobre filtrar inativas e
				// ordenar, mas pedia Tags:["core"] e esperava de volta esta
				// memória marcada "other" — ou seja, afirmava o defeito que a
				// busca tinha: tag como peso, não como recorte. Com o filtro
				// correto ela não entraria no conjunto, e o teste perderia o
				// segundo resultado de que precisa para exercitar a ordenação.
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
			{
				ID:         "expired-high-score",
				Content:    "postgres memory integrity",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceCritical,
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				ValidTo:    &expiredAt,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
			{
				ID:           "superseded-high-score",
				Content:      "postgres memory integrity",
				Category:     domain.CategoryKnowledge,
				Importance:   domain.ImportanceCritical,
				Tags:         []string{"core"},
				MemoryType:   domain.MemoryTypeSemantic,
				SupersededBy: "replacement",
				CreatedAt:    now,
				UpdatedAt:    now,
				DecayRate:    GetDecayRate(domain.MemoryTypeSemantic),
			},
			{
				ID:         "best-active",
				Content:    "postgres memory integrity",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceCritical,
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
		},
	}
	svc := &MemoryService{
		memoryRepo: repo,
		piiService: NewPIIService(),
	}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Query: "postgres memory integrity",
		Tags:  []string{"core"},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected 2 active results, got %d: %#v", resp.Total, resp.Results)
	}
	if resp.Results[0].ID != "best-active" {
		t.Fatalf("expected best-active first, got %s", resp.Results[0].ID)
	}
	if resp.Results[1].ID != "lower-score" {
		t.Fatalf("expected lower-score second, got %s", resp.Results[1].ID)
	}
	if len(repo.fullTextQueries) != 1 || repo.fullTextQueries[0] != "postgres memory integrity" {
		t.Fatalf("full-text queries = %v", repo.fullTextQueries)
	}
}

func TestSearchMemories_DeduplicatesVectorAndTextResults(t *testing.T) {
	now := time.Now()
	repo := &fakeMemoryRepository{
		byID: map[string]*domain.Memory{
			"vector-duplicate": {
				ID:         "vector-duplicate",
				Content:    "postgres memory integrity vector duplicate",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceCritical,
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
			"vector-only": {
				ID:         "vector-only",
				Content:    "postgres memory integrity vector only",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceImportant,
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
		},
		fullTextResults: []domain.Memory{
			{
				ID:         "vector-duplicate",
				Content:    "postgres memory integrity vector duplicate",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceCritical,
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
			{
				ID:         "text-only",
				Content:    "postgres memory integrity text only",
				Category:   domain.CategoryKnowledge,
				Importance: domain.ImportanceMinor,
				Tags:       []string{"core"},
				MemoryType: domain.MemoryTypeSemantic,
				CreatedAt:  now,
				UpdatedAt:  now,
				DecayRate:  GetDecayRate(domain.MemoryTypeSemantic),
			},
		},
	}
	svc := &MemoryService{
		memoryRepo:       repo,
		memoryGraphRepo:  &fakeMemoryGraphRepository{ids: []string{"vector-duplicate", "vector-only"}, scores: []float64{0.95, 0.9}},
		embeddingService: fakeEmbeddingGenerator{api: true},
		piiService:       NewPIIService(),
	}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Query: "postgres memory integrity",
		Tags:  []string{"core"},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}

	gotIDs := make([]string, 0, len(resp.Results))
	seen := make(map[string]bool, len(resp.Results))
	for _, result := range resp.Results {
		gotIDs = append(gotIDs, result.ID)
		if seen[result.ID] {
			t.Fatalf("duplicate result %q in %v", result.ID, gotIDs)
		}
		seen[result.ID] = true
	}

	wantIDs := []string{"vector-duplicate", "vector-only", "text-only"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("result IDs = %v, want %v", gotIDs, wantIDs)
	}
}

// --- Deterministic retrieval (RFC-014 fatia 1) ---

// countingEmbedder fails the test if it is used at all. The point of the
// exact route is not that embedding is wasteful there — it is that ranking a
// known key by similarity can return a DIFFERENT memory, which would make the
// audit compare the wrong fact against its source.
type countingEmbedder struct {
	t     *testing.T
	calls int
}

func (c *countingEmbedder) Embed(_ string) []float32 {
	c.calls++
	c.t.Error("exact search must not generate an embedding")
	return []float32{1, 0, 0}
}

func (c *countingEmbedder) HasAPI() bool { return true }

func TestSearchMemories_ExactBySourceReferenceSkipsEmbedding(t *testing.T) {
	embedder := &countingEmbedder{t: t}
	repo := &fakeMemoryRepository{
		exactResults: []domain.Memory{
			{ID: "m1", Content: "recusou SKU-9: nao trabalha com a marca", SourceReference: "decisao:123"},
		},
	}
	svc := &MemoryService{memoryRepo: repo, embeddingService: embedder}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		SourceReference: "decisao:123",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(resp.Results) != 1 || resp.Results[0].ID != "m1" {
		t.Fatalf("expected the memory of decisao:123, got %+v", resp.Results)
	}
	if embedder.calls != 0 {
		t.Errorf("embedding was called %d times on an exact lookup", embedder.calls)
	}
	if len(repo.fullTextQueries) != 0 {
		t.Errorf("exact lookup must not fall back to full-text search: %v", repo.fullTextQueries)
	}
	if len(repo.exactCalls) != 1 {
		t.Fatalf("expected exactly one exact lookup, got %d", len(repo.exactCalls))
	}
	if repo.exactCalls[0].SourceReference != "decisao:123" {
		t.Errorf("filter carried %q", repo.exactCalls[0].SourceReference)
	}
}

func TestSearchMemories_ExactByMetadataSkipsEmbedding(t *testing.T) {
	embedder := &countingEmbedder{t: t}
	repo := &fakeMemoryRepository{}
	svc := &MemoryService{memoryRepo: repo, embeddingService: embedder}

	if _, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Metadata: map[string]string{"cliente": "acme"},
	}); err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(repo.exactCalls) != 1 {
		t.Fatalf("expected one exact lookup, got %d", len(repo.exactCalls))
	}
	if repo.exactCalls[0].Metadata["cliente"] != "acme" {
		t.Errorf("metadata filter not propagated: %+v", repo.exactCalls[0].Metadata)
	}
}

// The narrow guard: a plain query must keep taking the semantic route.
func TestSearchMemories_WithoutExactFilterStaysSemantic(t *testing.T) {
	repo := &fakeMemoryRepository{
		fullTextResults: []domain.Memory{{ID: "m1", Content: "algo"}},
	}
	svc := &MemoryService{memoryRepo: repo, embeddingService: fakeEmbeddingGenerator{api: false}}

	if _, err := svc.SearchMemories(context.Background(), dto.SearchRequest{Query: "marca"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(repo.exactCalls) != 0 {
		t.Errorf("a plain query must not take the exact route")
	}
}

func TestSearchRequest_IsExact(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  dto.SearchRequest
		want bool
	}{
		{"source reference", dto.SearchRequest{SourceReference: "decisao:1"}, true},
		{"metadata", dto.SearchRequest{Metadata: map[string]string{"k": "v"}}, true},
		{"both", dto.SearchRequest{SourceReference: "d:1", Metadata: map[string]string{"k": "v"}}, true},
		{"query only", dto.SearchRequest{Query: "marca"}, false},
		{"empty metadata map", dto.SearchRequest{Metadata: map[string]string{}}, false},
		{"nothing", dto.SearchRequest{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.IsExact(); got != tc.want {
				t.Errorf("IsExact() = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- Batch expire ---

func TestBatchExpireMemories_RequiresSelectorAndReason(t *testing.T) {
	svc := &MemoryService{memoryRepo: &fakeMemoryRepository{}}

	if _, err := svc.BatchExpireMemories(context.Background(), dto.BatchExpireRequest{Reason: "x"}); err == nil {
		t.Error("expiring with no ids and no sourceReference must be refused — it would match nothing or everything")
	}
	if _, err := svc.BatchExpireMemories(context.Background(), dto.BatchExpireRequest{Ids: []string{"m1"}}); err == nil {
		t.Error("a bulk revocation with no reason is unauditable and must be refused")
	}
}

func TestBatchExpireMemories_PassesSelectorThrough(t *testing.T) {
	repo := &fakeMemoryRepository{}
	svc := &MemoryService{memoryRepo: repo}

	result, err := svc.BatchExpireMemories(context.Background(), dto.BatchExpireRequest{
		Ids:    []string{"m1", "m2"},
		Reason: "decisao revertida",
	})
	if err != nil {
		t.Fatalf("batch expire: %v", err)
	}
	if result.Expired != 2 {
		t.Errorf("expected 2 expired, got %d", result.Expired)
	}
	if len(repo.expireCalls) != 1 || repo.expireCalls[0].Reason != "decisao revertida" {
		t.Errorf("reason not propagated: %+v", repo.expireCalls)
	}
}

// --- Dedup escopado e idempotência (correções pré-Fatia B) ---

// O caso que motivou a correção: os fatos do VendaX são templates preenchidos
// por evento, um por cliente, e o texto de dois clientes diferentes é
// frequentemente IDÊNTICO. Antes, o segundo POST caía em distância 0, não
// criava nada e devolvia o id de uma memória do OUTRO cliente — falha
// silenciosa no eixo mais sensível da integração.
func TestCreateMemory_IdenticalContentDifferentClientsCoexist(t *testing.T) {
	repo := &fakeMemoryRepository{}
	svc := &MemoryService{memoryRepo: repo}

	const fato = "recusou SKU-9182: nao trabalha com a marca"

	first, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: fato,
		Tags:    []string{"cliente:acme-001", "tipo:recusa"},
	})
	if err != nil {
		t.Fatalf("primeira memoria: %v", err)
	}

	second, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: fato,
		Tags:    []string{"cliente:beta-002", "tipo:recusa"},
	})
	if err != nil {
		t.Fatalf("segunda memoria: %v", err)
	}

	if first.ID == second.ID {
		t.Fatal("clientes diferentes com texto identico foram deduplicados — o fato do segundo cliente sumiu")
	}
	if len(repo.byID) != 2 {
		t.Errorf("esperava 2 memorias, tem %d", len(repo.byID))
	}
	// Cada uma tem que ficar com a SUA tag de cliente.
	if !hasAllTags(first.Tags, []string{"cliente:acme-001"}) ||
		!hasAllTags(second.Tags, []string{"cliente:beta-002"}) {
		t.Errorf("tags trocadas: %v / %v", first.Tags, second.Tags)
	}
}

// O comportamento que continua desejável: o MESMO cliente reescrevendo o
// MESMO fato ainda deduplica. O defeito era o escopo, não a existência.
func TestCreateMemory_SameContentSameClientStillDeduplicates(t *testing.T) {
	repo := &fakeMemoryRepository{}
	svc := &MemoryService{memoryRepo: repo}

	const fato = "recusou SKU-9182: nao trabalha com a marca"
	tags := []string{"cliente:acme-001", "tipo:recusa"}

	first, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{Content: fato, Tags: tags})
	if err != nil {
		t.Fatalf("primeira: %v", err)
	}
	second, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{Content: fato, Tags: tags})
	if err != nil {
		t.Fatalf("segunda: %v", err)
	}

	if first.ID != second.ID {
		t.Error("mesmo cliente e mesmo texto deveriam deduplicar")
	}
	if len(repo.byID) != 1 {
		t.Errorf("esperava 1 memoria, tem %d", len(repo.byID))
	}
}

// O escopo pedido ao repositório precisa vir das tags da REQUISIÇÃO, não das
// da memória: o compressor acrescenta tags do LLM antes do dedup, e elas não
// são determinísticas — escopar por elas faria o mesmo fato não deduplicar
// contra si mesmo.
func TestCreateMemory_DedupScopeUsesRequestTags(t *testing.T) {
	repo := &fakeMemoryRepository{}
	svc := &MemoryService{memoryRepo: repo}

	tags := []string{"cliente:acme-001", "tipo:recusa"}
	if _, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: "algum fato", Tags: tags,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(repo.dedupFilters) != 1 {
		t.Fatalf("esperava uma consulta de dedup, houve %d", len(repo.dedupFilters))
	}
	if !reflect.DeepEqual(repo.dedupFilters[0].ScopeTags, tags) {
		t.Errorf("escopo = %v, esperado %v", repo.dedupFilters[0].ScopeTags, tags)
	}
	if repo.dedupFilters[0].SimHash == "" {
		t.Error("o filtro precisa carregar o SimHash para o pre-filtro por blocos")
	}
}

// Origem distinta = fato distinto por construção. Similaridade textual não
// tem voto, então o dedup nem é consultado.
func TestCreateMemory_SourceReferenceSkipsDedupEntirely(t *testing.T) {
	repo := &fakeMemoryRepository{}
	svc := &MemoryService{memoryRepo: repo}

	if _, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: "recusou SKU-9182", SourceReference: "decisao:1",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: "recusou SKU-9182", SourceReference: "decisao:2",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(repo.dedupFilters) != 0 {
		t.Errorf("com sourceReference o dedup nao deveria ser consultado; foi %d vez(es)", len(repo.dedupFilters))
	}
	if len(repo.byID) != 2 {
		t.Errorf("origens distintas devem produzir 2 memorias, tem %d", len(repo.byID))
	}
}

// O outbox do Core é at-least-once: a mesma origem chegando duas vezes tem que
// devolver o MESMO id, nunca criar um segundo fato.
func TestCreateMemory_SameSourceReferenceIsIdempotent(t *testing.T) {
	repo := &fakeMemoryRepository{bySourceRef: map[string]*domain.Memory{}}
	svc := &MemoryService{memoryRepo: repo}

	first, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: "recusou SKU-9182", SourceReference: "decisao:8f2a",
	})
	if err != nil {
		t.Fatalf("primeira: %v", err)
	}
	// Simula o que o repositório real faria: a origem passa a resolver para a
	// memória criada.
	repo.bySourceRef["decisao:8f2a"] = first

	second, err := svc.CreateMemory(context.Background(), dto.CreateMemoryRequest{
		Content: "recusou SKU-9182", SourceReference: "decisao:8f2a",
	})
	if err != nil {
		t.Fatalf("retentativa: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("a retentativa criou outro fato (%s != %s)", first.ID, second.ID)
	}
	if len(repo.byID) != 1 {
		t.Errorf("esperava 1 memoria apos a retentativa, tem %d", len(repo.byID))
	}
}

// --- Escopo por tag na BUSCA (vazamento entre clientes) ---

// O pior defeito possível segundo a RFC-014 §6.1: um recall que atravessa
// clientes entrega informação de um na conversa de outro. Observado em
// produção — `tags` entrava só no cálculo de pontuação, então a memória do
// outro cliente vinha junto, apenas ranqueada abaixo.
//
// A consulta usada aqui CASA com o conteúdo de propósito. Com consulta
// genérica o resultado seria 0 e a asserção "não veio o B" ficaria verde sem
// nada ter sido filtrado — um teste que passa por acidente. Por isso o teste
// afirma primeiro que o resultado NÃO é vazio.
func TestSearchMemories_TagIsFilterNotJustScore(t *testing.T) {
	const fato = "nao trabalha com a marca do produto"
	repo := &fakeMemoryRepository{
		fullTextResults: []domain.Memory{
			{ID: "a", Content: fato, Tags: []string{"cliente:vendax-a", "tipo:recusa"}},
			{ID: "b", Content: fato, Tags: []string{"cliente:vendax-b", "tipo:recusa"}},
		},
	}
	svc := &MemoryService{memoryRepo: repo}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Query: "nao trabalha com a marca",
		Tags:  []string{"cliente:vendax-a"},
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("busca: %v", err)
	}

	if resp.Total == 0 {
		t.Fatal("resultado vazio — a consulta precisa casar, senão o teste vira tautologia")
	}
	if resp.Total != 1 {
		t.Fatalf("esperava 1 resultado, veio %d: %+v", resp.Total, resp.Results)
	}
	if resp.Results[0].ID != "a" {
		t.Errorf("voltou a memória do cliente errado: %s", resp.Results[0].ID)
	}
	// E o recorte tem que chegar ao repositório, não ser aplicado depois.
	if len(repo.fullTextTagScopes) == 0 || !reflect.DeepEqual(repo.fullTextTagScopes[0], []string{"cliente:vendax-a"}) {
		t.Errorf("o filtro precisa ir para a consulta; escopos recebidos: %v", repo.fullTextTagScopes)
	}
}

// Duas tags exigem as duas: uma busca por cliente:A + tipo:recusa não pode
// devolver tudo que é tipo:recusa.
func TestSearchMemories_MultipleTagsRequireAll(t *testing.T) {
	const fato = "nao trabalha com a marca do produto"
	repo := &fakeMemoryRepository{
		fullTextResults: []domain.Memory{
			{ID: "recusa-a", Content: fato, Tags: []string{"cliente:vendax-a", "tipo:recusa"}},
			{ID: "compra-a", Content: fato, Tags: []string{"cliente:vendax-a", "tipo:compra"}},
			{ID: "recusa-b", Content: fato, Tags: []string{"cliente:vendax-b", "tipo:recusa"}},
		},
	}
	svc := &MemoryService{memoryRepo: repo}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Query: "nao trabalha com a marca",
		Tags:  []string{"cliente:vendax-a", "tipo:recusa"},
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("busca: %v", err)
	}
	if resp.Total != 1 || resp.Results[0].ID != "recusa-a" {
		t.Errorf("esperava só recusa-a, veio %d: %+v", resp.Total, resp.Results)
	}
}

// Sem tags o comportamento é o de hoje: sem recorte. Não mudar esse caso é
// parte do contrato — a Fatia A já está escrita contra ele.
func TestSearchMemories_NoTagsMeansNoScope(t *testing.T) {
	const fato = "nao trabalha com a marca do produto"
	repo := &fakeMemoryRepository{
		fullTextResults: []domain.Memory{
			{ID: "a", Content: fato, Tags: []string{"cliente:vendax-a"}},
			{ID: "b", Content: fato, Tags: []string{"cliente:vendax-b"}},
		},
	}
	svc := &MemoryService{memoryRepo: repo}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Query: "nao trabalha com a marca", Limit: 5,
	})
	if err != nil {
		t.Fatalf("busca: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("sem tags deveria trazer os dois, veio %d", resp.Total)
	}
}

// Não filtrar demais: a memória do cliente certo continua voltando quando ela
// tem tags ALÉM das pedidas.
func TestSearchMemories_ExtraTagsOnMemoryStillMatch(t *testing.T) {
	const fato = "nao trabalha com a marca do produto"
	repo := &fakeMemoryRepository{
		fullTextResults: []domain.Memory{
			{ID: "a", Content: fato, Tags: []string{"cliente:vendax-a", "tipo:recusa", "vendedor:v-77"}},
		},
	}
	svc := &MemoryService{memoryRepo: repo}

	resp, err := svc.SearchMemories(context.Background(), dto.SearchRequest{
		Query: "nao trabalha com a marca",
		Tags:  []string{"cliente:vendax-a"},
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("busca: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("tags extras na memória não podem excluí-la; veio %d", resp.Total)
	}
}
