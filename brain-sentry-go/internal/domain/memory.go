package domain

import (
	"encoding/json"
	"math"
	"strconv"
	"time"
)

// Memory is the core entity with vector embeddings for semantic search.
type Memory struct {
	ID                  string           `json:"id" db:"id"`
	Content             string           `json:"content" db:"content"`
	Summary             string           `json:"summary,omitempty" db:"summary"`
	Category            MemoryCategory   `json:"category,omitempty" db:"category"`
	Importance          ImportanceLevel  `json:"importance,omitempty" db:"importance"`
	ValidationStatus    ValidationStatus `json:"validationStatus,omitempty" db:"validation_status"`
	Embedding           []float32        `json:"-" db:"embedding"`
	Metadata            json.RawMessage  `json:"metadata,omitempty" db:"metadata"`
	Tags                []string         `json:"tags,omitempty" db:"-"`
	SourceType          string           `json:"sourceType,omitempty" db:"source_type"`
	SourceReference     string           `json:"sourceReference,omitempty" db:"source_reference"`
	CreatedBy           string           `json:"createdBy,omitempty" db:"created_by"`
	TenantID            string           `json:"tenantId" db:"tenant_id"`
	CreatedAt           time.Time        `json:"createdAt" db:"created_at"`
	UpdatedAt           time.Time        `json:"updatedAt" db:"updated_at"`
	LastAccessedAt      *time.Time       `json:"lastAccessedAt,omitempty" db:"last_accessed_at"`
	Version             int              `json:"version" db:"version"`
	AccessCount         int              `json:"accessCount" db:"access_count"`
	InjectionCount      int              `json:"injectionCount" db:"injection_count"`
	HelpfulCount        int              `json:"helpfulCount" db:"helpful_count"`
	NotHelpfulCount     int              `json:"notHelpfulCount" db:"not_helpful_count"`
	CodeExample         string           `json:"codeExample,omitempty" db:"code_example"`
	ProgrammingLanguage string           `json:"programmingLanguage,omitempty" db:"programming_language"`
	MemoryType          MemoryType       `json:"memoryType,omitempty" db:"memory_type"`
	DeletedAt           *time.Time       `json:"deletedAt,omitempty" db:"deleted_at"`
	EmotionalWeight     float64          `json:"emotionalWeight" db:"emotional_weight"`
	SimHash             string           `json:"simHash,omitempty" db:"sim_hash"`
	ValidFrom           *time.Time       `json:"validFrom,omitempty" db:"valid_from"`
	ValidTo             *time.Time       `json:"validTo,omitempty" db:"valid_to"`
	DecayRate           float64          `json:"decayRate" db:"decay_rate"`
	SupersededBy        string           `json:"supersededBy,omitempty" db:"superseded_by"`
	RecordedAt          time.Time        `json:"recordedAt" db:"recorded_at"`
	Provenance          Provenance       `json:"provenance,omitempty" db:"provenance"`
}

// HelpfulnessRate returns the ratio of helpful feedback.
func (m *Memory) HelpfulnessRate() float64 {
	total := m.HelpfulCount + m.NotHelpfulCount
	if total == 0 {
		return 0
	}
	return float64(m.HelpfulCount) / float64(total)
}

// RelevanceScore combines access frequency, injection rate, and helpfulness.
func (m *Memory) RelevanceScore() float64 {
	accessScore := float64(m.AccessCount) * 0.3
	injectionScore := float64(m.InjectionCount) * 0.4
	helpfulScore := m.HelpfulnessRate() * 0.3
	return accessScore + injectionScore + helpfulScore
}

// TrustReport is a consolidated, explainable confidence assessment for a
// memory. Score is the single number a consumer can rank/threshold on;
// Label buckets it for display; Reasons lists the factors that moved the
// score so the verdict is auditable rather than a black box.
type TrustReport struct {
	Score   float64  `json:"score"`   // 0.0 (do not trust) .. 1.0 (fully trusted)
	Label   string   `json:"label"`   // "high" | "medium" | "low"
	Reasons []string `json:"reasons"` // human-readable factors, highest-impact first
}

// TrustScore blends the signals brain-sentry already tracks — provenance,
// validation status, feedback, age and supersession — into one auditable
// confidence figure. It is a PURE function of the memory + a clock, so it
// is fully unit-testable without a database.
//
// Pipeline: start from provenance base trust, then apply additive
// adjustments, then hard caps (a superseded memory can never be "high"),
// then clamp to [0,1].
func (m *Memory) TrustScore(now time.Time) TrustReport {
	reasons := make([]string, 0, 5)

	// 1. Base trust from provenance.
	prov := m.Provenance
	score := prov.BaseTrust()
	if prov == "" {
		reasons = append(reasons, "provenance unknown (neutral base 0.50)")
	} else {
		reasons = append(reasons, "provenance "+string(prov)+" (base "+formatScore(prov.BaseTrust())+")")
	}

	// 2. Validation status adjustment. APPROVED/FLAGGED nudge additively;
	//    REJECTED is a human verdict of "this is wrong", so it's applied
	//    as a hard cap further down rather than a nudge that a high base
	//    could survive.
	switch m.ValidationStatus {
	case ValidationApproved:
		score += 0.10
		reasons = append(reasons, "validation APPROVED (+0.10)")
	case ValidationFlagged:
		score -= 0.20
		reasons = append(reasons, "validation FLAGGED (-0.20)")
	}

	// 3. Feedback adjustment — only when there's enough signal to matter.
	if total := m.HelpfulCount + m.NotHelpfulCount; total >= 2 {
		// Map helpfulness rate [0,1] to a [-0.15,+0.15] swing.
		delta := (m.HelpfulnessRate() - 0.5) * 0.30
		score += delta
		if delta >= 0 {
			reasons = append(reasons, "positive feedback (+"+formatScore(delta)+")")
		} else {
			reasons = append(reasons, "negative feedback ("+formatScore(delta)+")")
		}
	}

	// 4. Age decay — unvalidated memories lose a little trust as they age,
	//    on a gentle half-life of ~180 days. Validated/approved memories
	//    are exempt: a confirmed fact doesn't rot just because it's old.
	validated := m.Provenance == ProvenanceValidated || m.ValidationStatus == ValidationApproved
	if !validated {
		ref := m.RecordedAt
		if ref.IsZero() {
			ref = m.CreatedAt
		}
		if !ref.IsZero() {
			ageDays := now.Sub(ref).Hours() / 24.0
			if ageDays > 0 {
				const halfLifeDays = 180.0
				ageFactor := math.Pow(0.5, ageDays/halfLifeDays) // 1.0 → 0.5 over 180d
				penalty := score * (1 - ageFactor) * 0.30        // at most -30% of score
				if penalty > 0.001 {
					score -= penalty
					reasons = append(reasons, "age decay ("+formatScore(-penalty)+")")
				}
			}
		}
	}

	// 5. Hard caps — verdicts that no positive base should survive.
	//    A rejected memory was reviewed and found wrong; a superseded one
	//    is stale by definition. Both floor the score regardless of
	//    everything above.
	if m.ValidationStatus == ValidationRejected {
		if score > 0.20 {
			score = 0.20
		}
		reasons = append(reasons, "validation REJECTED — capped at 0.20")
	}
	if m.SupersededBy != "" {
		if score > 0.30 {
			score = 0.30
		}
		reasons = append(reasons, "superseded — capped at 0.30")
	}

	// Clamp.
	if score < 0 {
		score = 0
	} else if score > 1 {
		score = 1
	}

	return TrustReport{
		Score:   score,
		Label:   trustLabel(score),
		Reasons: reasons,
	}
}

func trustLabel(score float64) string {
	switch {
	case score >= 0.70:
		return "high"
	case score >= 0.40:
		return "medium"
	default:
		return "low"
	}
}

// formatScore renders a score delta with 2 decimals and a sign, for the
// human-readable Reasons list (e.g. "+0.10", "-0.40", "0.80").
func formatScore(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
