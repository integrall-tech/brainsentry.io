package domain

// ImportanceLevel indicates how strongly memories should be followed.
type ImportanceLevel string

const (
	ImportanceCritical  ImportanceLevel = "CRITICAL"
	ImportanceImportant ImportanceLevel = "IMPORTANT"
	ImportanceMinor     ImportanceLevel = "MINOR"
)

// MemoryCategory represents universal memory categories.
type MemoryCategory string

const (
	CategoryInsight      MemoryCategory = "INSIGHT"
	CategoryDecision     MemoryCategory = "DECISION"
	CategoryWarning      MemoryCategory = "WARNING"
	CategoryKnowledge    MemoryCategory = "KNOWLEDGE"
	CategoryAction       MemoryCategory = "ACTION"
	CategoryContext      MemoryCategory = "CONTEXT"
	CategoryReference    MemoryCategory = "REFERENCE"
	CategoryPattern      MemoryCategory = "PATTERN"
	CategoryAntipattern  MemoryCategory = "ANTIPATTERN"
	CategoryDomain       MemoryCategory = "DOMAIN"
	CategoryBug          MemoryCategory = "BUG"
	CategoryOptimization MemoryCategory = "OPTIMIZATION"
	CategoryIntegration  MemoryCategory = "INTEGRATION"
)

// ValidationStatus represents validation states for memories.
type ValidationStatus string

const (
	ValidationApproved ValidationStatus = "APPROVED"
	ValidationPending  ValidationStatus = "PENDING"
	ValidationFlagged  ValidationStatus = "FLAGGED"
	ValidationRejected ValidationStatus = "REJECTED"
)

// Provenance describes HOW a memory came to be known, which is the
// strongest single signal for how much to trust it. Modelled after the
// provenance taxonomy popularised by agent-memory systems: a directly
// stated fact is trusted more than something merely inferred from
// context. Feeds Memory.TrustScore().
type Provenance string

const (
	// ProvenanceExplicit — the user/agent stated this directly. High base trust.
	ProvenanceExplicit Provenance = "EXPLICIT"
	// ProvenanceValidated — independently confirmed (cross-checked, approved).
	ProvenanceValidated Provenance = "VALIDATED"
	// ProvenanceCorrected — rewritten after a contradiction was resolved.
	ProvenanceCorrected Provenance = "CORRECTED"
	// ProvenanceObserved — seen in the agent's actions/traces, not stated.
	ProvenanceObserved Provenance = "OBSERVED"
	// ProvenanceImported — ingested from an external source (file, API, sync).
	ProvenanceImported Provenance = "IMPORTED"
	// ProvenanceInferred — derived/guessed from surrounding context. Low base trust.
	ProvenanceInferred Provenance = "INFERRED"
)

// IsValid reports whether p is one of the known provenance values.
func (p Provenance) IsValid() bool {
	switch p {
	case ProvenanceExplicit, ProvenanceValidated, ProvenanceCorrected,
		ProvenanceObserved, ProvenanceImported, ProvenanceInferred:
		return true
	default:
		return false
	}
}

// BaseTrust returns the starting confidence [0,1] implied by provenance
// alone, before age / validation / feedback adjustments. An empty or
// unknown provenance falls back to a neutral 0.5.
func (p Provenance) BaseTrust() float64 {
	switch p {
	case ProvenanceValidated:
		return 0.90
	case ProvenanceExplicit:
		return 0.80
	case ProvenanceCorrected:
		return 0.75
	case ProvenanceObserved:
		return 0.60
	case ProvenanceImported:
		return 0.50
	case ProvenanceInferred:
		return 0.40
	default:
		return 0.50
	}
}

// RelationshipType represents memory relationship types.
type RelationshipType string

const (
	RelUsedWith      RelationshipType = "USED_WITH"
	RelConflictsWith RelationshipType = "CONFLICTS_WITH"
	RelSupersedes    RelationshipType = "SUPERSEDES"
	RelRelatedTo     RelationshipType = "RELATED_TO"
	RelRequires      RelationshipType = "REQUIRES"
	RelPartOf        RelationshipType = "PART_OF"
)

// MemoryType represents the cognitive type of a memory.
type MemoryType string

const (
	MemoryTypeSemantic    MemoryType = "SEMANTIC"    // Facts, concepts, knowledge
	MemoryTypeEpisodic    MemoryType = "EPISODIC"    // Events, experiences, sessions
	MemoryTypeProcedural  MemoryType = "PROCEDURAL"  // How-to, procedures, patterns
	MemoryTypeAssociative MemoryType = "ASSOCIATIVE" // Links between concepts
	MemoryTypePersonality MemoryType = "PERSONALITY" // Stable user traits, preferences, identity
	MemoryTypePreference  MemoryType = "PREFERENCE"  // User preferences, likes/dislikes
	MemoryTypeThread      MemoryType = "THREAD"      // Conversational thread context, ephemeral
	MemoryTypeTask        MemoryType = "TASK"         // Ongoing tasks, TODOs, action items
	MemoryTypeEmotion     MemoryType = "EMOTION"      // Emotional reactions, sentiments
)

// CorrectionStatus represents the status of a memory correction.
type CorrectionStatus string

const (
	CorrectionPending  CorrectionStatus = "PENDING"
	CorrectionApproved CorrectionStatus = "APPROVED"
	CorrectionRejected CorrectionStatus = "REJECTED"
	CorrectionApplied  CorrectionStatus = "APPLIED"
)

// NoteType represents types of notes.
type NoteType string

const (
	NoteInsight      NoteType = "INSIGHT"
	NoteHindsight    NoteType = "HINDSIGHT"
	NotePattern      NoteType = "PATTERN"
	NoteAntipattern  NoteType = "ANTIPATTERN"
	NoteArchitecture NoteType = "ARCHITECTURE"
	NoteIntegration  NoteType = "INTEGRATION"
)

// NoteCategory represents note scope.
type NoteCategory string

const (
	NoteCategoryProjectSpecific NoteCategory = "PROJECT_SPECIFIC"
	NoteCategoryShared          NoteCategory = "SHARED"
	NoteCategoryGeneric         NoteCategory = "GENERIC"
)

// ObservationType represents typed session observations.
type ObservationType string

const (
	ObservationDecision  ObservationType = "DECISION"
	ObservationBugfix    ObservationType = "BUGFIX"
	ObservationFeature   ObservationType = "FEATURE"
	ObservationRefactor  ObservationType = "REFACTOR"
	ObservationDiscovery ObservationType = "DISCOVERY"
	ObservationChange    ObservationType = "CHANGE"
)

// NoteSeverity represents severity levels for notes.
type NoteSeverity string

const (
	SeverityCritical NoteSeverity = "CRITICAL"
	SeverityHigh     NoteSeverity = "HIGH"
	SeverityMedium   NoteSeverity = "MEDIUM"
	SeverityLow      NoteSeverity = "LOW"
)

// Valid reports whether the level is one the system actually understands.
//
// Exists because these enums arrive as free-form strings over JSON and NOTHING
// validated them: a value outside the set was accepted, stored, and then
// matched no filter — the row existed and was invisible. Failing the request is
// cheaper than a fact nobody can find.
func (i ImportanceLevel) Valid() bool {
	switch i {
	case ImportanceCritical, ImportanceImportant, ImportanceMinor:
		return true
	}
	return false
}

// Valid reports whether the category is a known one.
func (c MemoryCategory) Valid() bool {
	switch c {
	case CategoryInsight, CategoryDecision, CategoryWarning, CategoryKnowledge,
		CategoryAction, CategoryContext, CategoryReference, CategoryPattern,
		CategoryAntipattern, CategoryDomain, CategoryBug:
		return true
	}
	return false
}

// Valid reports whether the provenance is a known one.
func (p Provenance) Valid() bool {
	switch p {
	case ProvenanceExplicit, ProvenanceValidated, ProvenanceCorrected,
		ProvenanceObserved, ProvenanceImported, ProvenanceInferred:
		return true
	}
	return false
}
