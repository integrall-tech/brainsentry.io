package domain

import (
	"testing"
	"time"
)

func TestProvenance_IsValid(t *testing.T) {
	valid := []Provenance{
		ProvenanceExplicit, ProvenanceValidated, ProvenanceCorrected,
		ProvenanceObserved, ProvenanceImported, ProvenanceInferred,
	}
	for _, p := range valid {
		if !p.IsValid() {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []Provenance{"", "BOGUS", "explicit"} {
		if p.IsValid() {
			t.Errorf("%q should be invalid", p)
		}
	}
}

func TestProvenance_BaseTrust_Ordering(t *testing.T) {
	// Validated > Explicit > Corrected > Observed > Imported > Inferred,
	// and unknown falls back to neutral 0.5.
	if !(ProvenanceValidated.BaseTrust() > ProvenanceExplicit.BaseTrust() &&
		ProvenanceExplicit.BaseTrust() > ProvenanceCorrected.BaseTrust() &&
		ProvenanceCorrected.BaseTrust() > ProvenanceObserved.BaseTrust() &&
		ProvenanceObserved.BaseTrust() > ProvenanceImported.BaseTrust() &&
		ProvenanceImported.BaseTrust() > ProvenanceInferred.BaseTrust()) {
		t.Error("provenance base trust is not strictly ordered as expected")
	}
	if got := Provenance("").BaseTrust(); got != 0.50 {
		t.Errorf("empty provenance base trust: got %v want 0.50", got)
	}
}

// fixedNow is a stable clock so age-based tests are deterministic.
var fixedNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func TestTrustScore_ValidatedFreshIsHigh(t *testing.T) {
	m := &Memory{
		Provenance:       ProvenanceValidated,
		ValidationStatus: ValidationApproved,
		RecordedAt:       fixedNow.Add(-24 * time.Hour),
	}
	r := m.TrustScore(fixedNow)
	if r.Label != "high" {
		t.Errorf("validated+approved fresh memory should be high; got %s (%.2f)", r.Label, r.Score)
	}
	if r.Score < 0.9 {
		t.Errorf("expected score >= 0.9; got %.2f", r.Score)
	}
}

func TestTrustScore_InferredIsLowerThanExplicit(t *testing.T) {
	base := Memory{RecordedAt: fixedNow}
	inferred := base
	inferred.Provenance = ProvenanceInferred
	explicit := base
	explicit.Provenance = ProvenanceExplicit

	si := inferred.TrustScore(fixedNow).Score
	se := explicit.TrustScore(fixedNow).Score
	if !(se > si) {
		t.Errorf("explicit (%.2f) should outscore inferred (%.2f)", se, si)
	}
}

func TestTrustScore_RejectedDrivesLow(t *testing.T) {
	m := &Memory{
		Provenance:       ProvenanceExplicit, // high base...
		ValidationStatus: ValidationRejected, // ...but rejected
		RecordedAt:       fixedNow,
	}
	r := m.TrustScore(fixedNow)
	if r.Score >= 0.40 {
		t.Errorf("rejected memory should not be medium/high; got %.2f", r.Score)
	}
	if r.Label != "low" {
		t.Errorf("rejected memory label: got %s want low", r.Label)
	}
}

func TestTrustScore_SupersededCappedAt030(t *testing.T) {
	m := &Memory{
		Provenance:       ProvenanceValidated, // would be ~0.9...
		ValidationStatus: ValidationApproved,
		RecordedAt:       fixedNow,
		SupersededBy:     "some-newer-id", // ...but superseded
	}
	r := m.TrustScore(fixedNow)
	if r.Score > 0.30 {
		t.Errorf("superseded memory must be capped at 0.30; got %.2f", r.Score)
	}
}

func TestTrustScore_PositiveFeedbackRaises(t *testing.T) {
	withFeedback := &Memory{
		Provenance:   ProvenanceObserved,
		RecordedAt:   fixedNow,
		HelpfulCount: 9, NotHelpfulCount: 1,
	}
	noFeedback := &Memory{Provenance: ProvenanceObserved, RecordedAt: fixedNow}
	if !(withFeedback.TrustScore(fixedNow).Score > noFeedback.TrustScore(fixedNow).Score) {
		t.Error("strong positive feedback should raise the trust score")
	}
}

func TestTrustScore_NegativeFeedbackLowers(t *testing.T) {
	bad := &Memory{
		Provenance:   ProvenanceObserved,
		RecordedAt:   fixedNow,
		HelpfulCount: 1, NotHelpfulCount: 9,
	}
	neutral := &Memory{Provenance: ProvenanceObserved, RecordedAt: fixedNow}
	if !(bad.TrustScore(fixedNow).Score < neutral.TrustScore(fixedNow).Score) {
		t.Error("mostly-unhelpful feedback should lower the trust score")
	}
}

func TestTrustScore_SingleVoteDoesNotMove(t *testing.T) {
	// Only >=2 votes count, so one lone vote shouldn't swing the score.
	one := &Memory{Provenance: ProvenanceObserved, RecordedAt: fixedNow, HelpfulCount: 1}
	none := &Memory{Provenance: ProvenanceObserved, RecordedAt: fixedNow}
	if one.TrustScore(fixedNow).Score != none.TrustScore(fixedNow).Score {
		t.Error("a single feedback vote should not move the score (needs >=2)")
	}
}

func TestTrustScore_AgeDecaysUnvalidated(t *testing.T) {
	old := &Memory{Provenance: ProvenanceObserved, RecordedAt: fixedNow.Add(-365 * 24 * time.Hour)}
	fresh := &Memory{Provenance: ProvenanceObserved, RecordedAt: fixedNow}
	if !(old.TrustScore(fixedNow).Score < fresh.TrustScore(fixedNow).Score) {
		t.Error("an old unvalidated memory should decay below a fresh one")
	}
}

func TestTrustScore_ValidatedDoesNotAgeDecay(t *testing.T) {
	old := &Memory{
		Provenance:       ProvenanceValidated,
		ValidationStatus: ValidationApproved,
		RecordedAt:       fixedNow.Add(-365 * 24 * time.Hour),
	}
	fresh := &Memory{
		Provenance:       ProvenanceValidated,
		ValidationStatus: ValidationApproved,
		RecordedAt:       fixedNow,
	}
	// Validated memories are exempt from age decay — scores should match.
	if old.TrustScore(fixedNow).Score != fresh.TrustScore(fixedNow).Score {
		t.Errorf("validated memory should not age-decay; old=%.4f fresh=%.4f",
			old.TrustScore(fixedNow).Score, fresh.TrustScore(fixedNow).Score)
	}
}

func TestTrustScore_AlwaysClampedAndLabelled(t *testing.T) {
	// A pathological stack of penalties must still yield a valid [0,1]
	// score and a non-empty label + reasons.
	m := &Memory{
		Provenance:       ProvenanceInferred,
		ValidationStatus: ValidationRejected,
		RecordedAt:       fixedNow.Add(-1000 * 24 * time.Hour),
		HelpfulCount:     0, NotHelpfulCount: 10,
		SupersededBy: "x",
	}
	r := m.TrustScore(fixedNow)
	if r.Score < 0 || r.Score > 1 {
		t.Errorf("score out of [0,1]: %.2f", r.Score)
	}
	if r.Label == "" || len(r.Reasons) == 0 {
		t.Error("trust report must always carry a label and reasons")
	}
}
