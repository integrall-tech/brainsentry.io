package dto

import (
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

// LoginResponse represents the authentication response.
type LoginResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refreshToken,omitempty"`
	User         UserResponse `json:"user"`
	TenantID     string       `json:"tenantId"`
}

// UserResponse represents a user in API responses.
type UserResponse struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Name  string   `json:"name,omitempty"`
	Roles []string `json:"roles"`
}

// MemoryResponse represents a single memory in API responses.
type MemoryResponse struct {
	ID                  string                  `json:"id"`
	Content             string                  `json:"content"`
	Summary             string                  `json:"summary,omitempty"`
	Category            domain.MemoryCategory   `json:"category,omitempty"`
	Importance          domain.ImportanceLevel  `json:"importance,omitempty"`
	ValidationStatus    domain.ValidationStatus `json:"validationStatus,omitempty"`
	Metadata            map[string]any          `json:"metadata,omitempty"`
	Tags                []string                `json:"tags,omitempty"`
	SourceType          string                  `json:"sourceType,omitempty"`
	SourceReference     string                  `json:"sourceReference,omitempty"`
	CreatedBy           string                  `json:"createdBy,omitempty"`
	TenantID            string                  `json:"tenantId"`
	CreatedAt           time.Time               `json:"createdAt"`
	UpdatedAt           time.Time               `json:"updatedAt"`
	LastAccessedAt      *time.Time              `json:"lastAccessedAt,omitempty"`
	Version             int                     `json:"version"`
	AccessCount         int                     `json:"accessCount"`
	InjectionCount      int                     `json:"injectionCount"`
	HelpfulCount        int                     `json:"helpfulCount"`
	NotHelpfulCount     int                     `json:"notHelpfulCount"`
	HelpfulnessRate     float64                 `json:"helpfulnessRate"`
	RelevanceScore      float64                 `json:"relevanceScore"`
	CodeExample         string                  `json:"codeExample,omitempty"`
	ProgrammingLanguage string                  `json:"programmingLanguage,omitempty"`
	MemoryType          domain.MemoryType       `json:"memoryType,omitempty"`
	EmotionalWeight     float64                 `json:"emotionalWeight"`
	SimHash             string                  `json:"simHash,omitempty"`
	ValidFrom           *time.Time              `json:"validFrom,omitempty"`
	ValidTo             *time.Time              `json:"validTo,omitempty"`
	DecayRate           float64                 `json:"decayRate"`
	SupersededBy        string                  `json:"supersededBy,omitempty"`
	DecayedRelevance    float64                 `json:"decayedRelevance"`
	Provenance          domain.Provenance       `json:"provenance,omitempty"`
	Trust               *domain.TrustReport     `json:"trust,omitempty"`
	ScoreTrace          *ScoreTraceResponse     `json:"scoreTrace,omitempty"`
	RelatedMemories     []RelatedMemoryRef      `json:"relatedMemories,omitempty"`
}

// RelatedMemoryRef represents a reference to a related memory.
type RelatedMemoryRef struct {
	ID               string                 `json:"id"`
	Summary          string                 `json:"summary,omitempty"`
	RelationshipType domain.RelationshipType `json:"relationshipType"`
	Strength         float64                `json:"strength"`
}

// MemoryListResponse represents a paginated list of memories.
type MemoryListResponse struct {
	Memories      []MemoryResponse `json:"memories"`
	Page          int              `json:"page"`
	Size          int              `json:"size"`
	TotalElements int64            `json:"totalElements"`
	TotalPages    int              `json:"totalPages"`
	HasNext       bool             `json:"hasNext"`
	HasPrevious   bool             `json:"hasPrevious"`
}

// SearchResponse wraps search results with timing and count per spec.
type SearchResponse struct {
	Results      []MemoryResponse `json:"results"`
	Total        int              `json:"total"`
	SearchTimeMs int64            `json:"searchTimeMs"`
}

// InterceptResponse represents the prompt interception result.
type InterceptResponse struct {
	Enhanced        bool              `json:"enhanced"`
	OriginalPrompt  string            `json:"originalPrompt"`
	EnhancedPrompt  string            `json:"enhancedPrompt,omitempty"`
	ContextInjected string            `json:"contextInjected,omitempty"`
	MemoriesUsed    []MemoryReference `json:"memoriesUsed,omitempty"`
	NotesUsed       []NoteReference   `json:"notesUsed,omitempty"`
	LatencyMs       int64             `json:"latencyMs"`
	Reasoning       string            `json:"reasoning,omitempty"`
	Confidence      float64           `json:"confidence"`
	TokensInjected  int               `json:"tokensInjected"`
	LLMCalls        int               `json:"llmCalls"`
}

// MemoryReference is a reference to a memory used in interception.
type MemoryReference struct {
	ID             string                 `json:"id"`
	Summary        string                 `json:"summary,omitempty"`
	Category       domain.MemoryCategory  `json:"category,omitempty"`
	Importance     domain.ImportanceLevel `json:"importance,omitempty"`
	RelevanceScore float64                `json:"relevanceScore"`
	Excerpt        string                 `json:"excerpt,omitempty"`
}

// NoteReference is a reference to a note used in interception.
type NoteReference struct {
	ID       string              `json:"id"`
	Title    string              `json:"title,omitempty"`
	Type     domain.NoteType     `json:"type,omitempty"`
	Severity domain.NoteSeverity `json:"severity,omitempty"`
	Excerpt  string              `json:"excerpt,omitempty"`
}

// StatsResponse represents system statistics.
type StatsResponse struct {
	TotalMemories        int64            `json:"totalMemories"`
	MemoriesByCategory   map[string]int64 `json:"memoriesByCategory"`
	MemoriesByImportance map[string]int64 `json:"memoriesByImportance"`
	RequestsToday        int64            `json:"requestsToday"`
	TotalInjections      int64            `json:"totalInjections"`
	InjectionRate        float64          `json:"injectionRate"`
	AvgLatencyMs         float64          `json:"avgLatencyMs"`
	HelpfulnessRate      float64          `json:"helpfulnessRate"`
	ActiveMemories24h    int64            `json:"activeMemories24h"`
}

// KnowledgeGraphResponse represents the knowledge graph visualization data.
type KnowledgeGraphResponse struct {
	Nodes      []EntityNode `json:"nodes"`
	Edges      []EntityEdge `json:"edges"`
	TotalNodes int          `json:"totalNodes"`
	TotalEdges int          `json:"totalEdges"`
}

// EntityNode represents a node in the knowledge graph.
type EntityNode struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	SourceMemoryID string            `json:"sourceMemoryId,omitempty"`
	Properties     map[string]string `json:"properties,omitempty"`
}

// EntityEdge represents an edge in the knowledge graph.
type EntityEdge struct {
	ID         string            `json:"id"`
	SourceID   string            `json:"sourceId"`
	TargetID   string            `json:"targetId"`
	SourceName string            `json:"sourceName,omitempty"`
	TargetName string            `json:"targetName,omitempty"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// AuditLogResponse represents an audit log entry.
type AuditLogResponse struct {
	ID               string         `json:"id"`
	EventType        string         `json:"eventType"`
	Timestamp        time.Time      `json:"timestamp"`
	UserID           string         `json:"userId,omitempty"`
	SessionID        string         `json:"sessionId,omitempty"`
	UserRequest      string         `json:"userRequest,omitempty"`
	Decision         map[string]any `json:"decision,omitempty"`
	Reasoning        string         `json:"reasoning,omitempty"`
	Confidence       *float64       `json:"confidence,omitempty"`
	InputData        map[string]any `json:"inputData,omitempty"`
	OutputData       map[string]any `json:"outputData,omitempty"`
	MemoriesAccessed []string       `json:"memoriesAccessed,omitempty"`
	MemoriesCreated  []string       `json:"memoriesCreated,omitempty"`
	MemoriesModified []string       `json:"memoriesModified,omitempty"`
	LatencyMs        *int           `json:"latencyMs,omitempty"`
	LLMCalls         *int           `json:"llmCalls,omitempty"`
	TokensUsed       *int           `json:"tokensUsed,omitempty"`
	Outcome          string         `json:"outcome,omitempty"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
	UserFeedback     map[string]any `json:"userFeedback,omitempty"`
	TenantID         string         `json:"tenantId"`
}

// SessionAnalysisResponse represents session analysis results.
type SessionAnalysisResponse struct {
	SessionID      string            `json:"sessionId"`
	TenantID       string            `json:"tenantId"`
	AnalyzedAt     time.Time         `json:"analyzedAt"`
	TotalDecisions int               `json:"totalDecisions"`
	TotalInsights  int               `json:"totalInsights"`
	TotalFailures  int               `json:"totalFailures"`
	Decisions      []Decision        `json:"decisions,omitempty"`
	Insights       []Insight         `json:"insights,omitempty"`
	Failures       []FailureInsight  `json:"failures,omitempty"`
}

// Decision from session analysis.
type Decision struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Rationale   string `json:"rationale,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	Context     string `json:"context,omitempty"`
}

// Insight from session analysis.
type Insight struct {
	Category   string `json:"category"`
	Content    string `json:"content"`
	Importance string `json:"importance,omitempty"`
	RelatedTo  string `json:"relatedTo,omitempty"`
}

// FailureInsight from session analysis.
type FailureInsight struct {
	ErrorType      string `json:"errorType"`
	ErrorMessage   string `json:"errorMessage"`
	Context        string `json:"context,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	LessonsLearned string `json:"lessonsLearned,omitempty"`
	PreventionHint string `json:"preventionHint,omitempty"`
}

// SessionObservationResponse represents a typed session observation.
type SessionObservationResponse struct {
	ID              string                `json:"id"`
	SessionID       string                `json:"sessionId"`
	Type            domain.ObservationType `json:"type"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Context         string                `json:"context,omitempty"`
	RelatedMemoryID string                `json:"relatedMemoryId,omitempty"`
	CreatedAt       time.Time             `json:"createdAt"`
	AutoGenerated   bool                  `json:"autoGenerated"`
}

// ScoreTraceResponse provides an explainable breakdown of the hybrid scoring.
type ScoreTraceResponse struct {
	FinalScore      float64 `json:"finalScore"`
	SimBoost        float64 `json:"simBoost"`
	TokenOverlap    float64 `json:"tokenOverlap"`
	GraphProximity  float64 `json:"graphProximity"`
	RecencyScore    float64 `json:"recencyScore"`
	TagMatchScore   float64 `json:"tagMatchScore"`
	ImportanceScore float64 `json:"importanceScore"`
	DecayFactor     float64 `json:"decayFactor"`
	EmotionalBoost  float64 `json:"emotionalBoost"`
}

// ErrorResponse represents a structured API error per spec.
type ErrorResponse struct {
	Error         string `json:"error"`
	Message       string `json:"message,omitempty"`
	Status        int    `json:"status"`
	ErrorCode     string `json:"errorCode,omitempty"`
	ErrorCategory string `json:"errorCategory,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}
