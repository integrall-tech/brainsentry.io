package dto

import (
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

// LoginRequest represents authentication credentials.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateMemoryRequest represents a request to create a new memory.
type CreateMemoryRequest struct {
	Content             string                 `json:"content" validate:"required,max=10000"`
	Summary             string                 `json:"summary,omitempty" validate:"max=500"`
	Category            domain.MemoryCategory  `json:"category,omitempty"`
	Importance          domain.ImportanceLevel `json:"importance,omitempty"`
	MemoryType          domain.MemoryType      `json:"memoryType,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Metadata            map[string]any         `json:"metadata,omitempty"`
	SourceType          string                 `json:"sourceType,omitempty"`
	SourceReference     string                 `json:"sourceReference,omitempty"`
	CodeExample         string                 `json:"codeExample,omitempty" validate:"max=5000"`
	ProgrammingLanguage string                 `json:"programmingLanguage,omitempty"`
	CreatedBy           string                 `json:"createdBy,omitempty"`
	TenantID            string                 `json:"tenantId,omitempty"`
	EmotionalWeight     *float64               `json:"emotionalWeight,omitempty"`
	ValidFrom           *time.Time             `json:"validFrom,omitempty"`
	ValidTo             *time.Time             `json:"validTo,omitempty"`
	Provenance          domain.Provenance      `json:"provenance,omitempty"`
}

// FlagMemoryRequest represents a request to flag a memory as incorrect.
type FlagMemoryRequest struct {
	Reason           string `json:"reason" validate:"required"`
	CorrectedContent string `json:"correctedContent,omitempty"`
	FlaggedBy        string `json:"flaggedBy,omitempty"`
}

// ReviewCorrectionRequest represents a request to review a flagged correction.
type ReviewCorrectionRequest struct {
	Action      string `json:"action" validate:"required"` // "approve" or "reject"
	ReviewNotes string `json:"reviewNotes,omitempty"`
	ReviewedBy  string `json:"reviewedBy,omitempty"`
}

// UpdateMemoryRequest represents a request to update a memory.
type UpdateMemoryRequest struct {
	Content             string                 `json:"content,omitempty" validate:"max=10000"`
	Summary             string                 `json:"summary,omitempty" validate:"max=500"`
	Category            domain.MemoryCategory  `json:"category,omitempty"`
	Importance          domain.ImportanceLevel `json:"importance,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Metadata            map[string]any         `json:"metadata,omitempty"`
	CodeExample         string                 `json:"codeExample,omitempty" validate:"max=5000"`
	ProgrammingLanguage string                 `json:"programmingLanguage,omitempty"`
	ChangeReason        string                 `json:"changeReason,omitempty"`
}

// SearchRequest represents a memory search query.
type SearchRequest struct {
	Query          string                  `json:"query"`
	Categories     []domain.MemoryCategory `json:"categories,omitempty"`
	MinImportance  domain.ImportanceLevel  `json:"minImportance,omitempty"`
	Tags           []string                `json:"tags,omitempty"`
	Limit          int                     `json:"limit,omitempty"`
	IncludeRelated bool                    `json:"includeRelated,omitempty"`
	TenantID       string                  `json:"tenantId,omitempty"`

	// SourceReference and Metadata switch the search to a DETERMINISTIC
	// route: exact match, ordered by created_at desc, with no embedding, no
	// query expansion and no similarity ranking (RFC-014 fatia 1).
	//
	// They exist because the audit asks "give me the fact produced by event
	// X" — a lookup by identity. Answering that with semantic search would
	// cost an embedding call and could return a *different* memory that
	// happens to sit nearby in vector space.
	//
	// When either is set, Query becomes optional.
	SourceReference string            `json:"sourceReference,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// IsExact reports whether this request must take the deterministic route.
func (r SearchRequest) IsExact() bool {
	return r.SourceReference != "" || len(r.Metadata) > 0
}

// BatchExpireRequest closes the validity window of many memories at once.
// Either Ids or SourceReference must be given; Reason is recorded on each
// memory so a revocation explains itself.
type BatchExpireRequest struct {
	Ids             []string `json:"ids,omitempty"`
	SourceReference string   `json:"sourceReference,omitempty"`
	Reason          string   `json:"reason"`
}

// InterceptRequest is the main entry point for context injection.
type InterceptRequest struct {
	Prompt            string         `json:"prompt" validate:"required"`
	UserID            string         `json:"userId,omitempty"`
	SessionID         string         `json:"sessionId,omitempty"`
	Context           map[string]any `json:"context,omitempty"`
	TenantID          string         `json:"tenantId,omitempty"`
	MaxTokens         int            `json:"maxTokens,omitempty"`
	ForceDeepAnalysis bool           `json:"forceDeepAnalysis,omitempty"`
}

// SessionAnalysisRequest represents a session analysis request.
type SessionAnalysisRequest struct {
	SessionID        string `json:"sessionId" validate:"required"`
	TenantID         string `json:"tenantId" validate:"required"`
	IncludeFailures  *bool  `json:"includeFailures,omitempty"`
	IncludeDecisions *bool  `json:"includeDecisions,omitempty"`
	IncludeInsights  *bool  `json:"includeInsights,omitempty"`
	MaxInsights      int    `json:"maxInsights,omitempty"`
	FromTimestamp    *int64 `json:"fromTimestamp,omitempty"`
	ToTimestamp      *int64 `json:"toTimestamp,omitempty"`
}

// CreateHindsightNoteRequest for manual hindsight note creation.
type CreateHindsightNoteRequest struct {
	SessionID           string   `json:"sessionId" validate:"required"`
	ErrorType           string   `json:"errorType" validate:"required"`
	ErrorMessage        string   `json:"errorMessage" validate:"required"`
	ErrorContext        string   `json:"errorContext,omitempty"`
	Resolution          string   `json:"resolution,omitempty"`
	ResolutionSteps     string   `json:"resolutionSteps,omitempty"`
	ResolutionReference string   `json:"resolutionReference,omitempty"`
	LessonsLearned      string   `json:"lessonsLearned,omitempty"`
	PreventionStrategy  string   `json:"preventionStrategy,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	RelatedMemoryIDs    []string `json:"relatedMemoryIds,omitempty"`
	Priority            string   `json:"priority,omitempty"`
}

// CompressionRequest for context compression.
type CompressionRequest struct {
	Messages         []CompressionMessage `json:"messages" validate:"required"`
	TokenThreshold   int                  `json:"tokenThreshold,omitempty"`
	PreserveRecent   int                  `json:"preserveRecent,omitempty"`
	TargetRatio      float64              `json:"targetRatio,omitempty"`
	ContextHint      string               `json:"contextHint,omitempty"`
	PreserveKeywords []string             `json:"preserveKeywords,omitempty"`
}

// CompressionMessage represents a message in a compression request.
type CompressionMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp,omitempty"`
	ToolName  string `json:"toolName,omitempty"`
}

// CreateTenantRequest for creating a new tenant.
type CreateTenantRequest struct {
	Name        string         `json:"name" validate:"required"`
	Slug        string         `json:"slug" validate:"required"`
	Description string         `json:"description,omitempty"`
	MaxMemories int            `json:"maxMemories,omitempty"`
	MaxUsers    int            `json:"maxUsers,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
}

// CreateUserRequest for creating a new user.
type CreateUserRequest struct {
	Email    string   `json:"email" validate:"required,email"`
	Name     string   `json:"name,omitempty"`
	Password string   `json:"password" validate:"required,min=8"`
	TenantID string   `json:"tenantId" validate:"required"`
	Roles    []string `json:"roles,omitempty"`
}
