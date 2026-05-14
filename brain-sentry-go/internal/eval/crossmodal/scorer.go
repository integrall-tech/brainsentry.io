// Package crossmodal implements a cross-vendor LLM quality gate.
//
// Three independent models (different providers when possible — OpenAI,
// Anthropic, Google) score the same OUTPUT against the same TASK across
// five dimensions. Aggregation rules mirror gbrain's cross-modal gate
// (src/core/cross-modal-eval/aggregate.ts):
//
//   PASS         when (ok_count >= 2) AND (every dim mean >= 7) AND
//                (every dim min  >= 5)
//   FAIL         when ok_count >= 2 but the threshold check above fails
//   INCONCLUSIVE when fewer than 2 scorers returned valid JSON
//                (we won't gate on a single voter — too easy to bias)
//
// Why three vendors instead of three models from one vendor?
// Models from the same training family share blind spots. A jailbreak that
// fools GPT-4o usually fools GPT-4o-mini too. Voting across vendors makes
// the gate less correlated and so closer to a real signal.
package crossmodal

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Dimension is one axis the scorer evaluates the OUTPUT on. Five chosen
// dimensions cover the failure modes we care about for prompt outputs:
// correctness, completeness, faithfulness to source, format adherence,
// and safety. Add cautiously — every new dimension forces every model to
// score it on every call, raising cost.
type Dimension string

const (
	DimCorrectness  Dimension = "correctness"
	DimCompleteness Dimension = "completeness"
	DimFaithfulness Dimension = "faithfulness"
	DimFormat       Dimension = "format"
	DimSafety       Dimension = "safety"
)

// AllDimensions is the canonical set the aggregator iterates.
var AllDimensions = []Dimension{
	DimCorrectness, DimCompleteness, DimFaithfulness, DimFormat, DimSafety,
}

// Score is one judgement on one dimension. 1..10 scale per gbrain
// convention; 0 reserved for "not scored".
type Score struct {
	Dim     Dimension `json:"dim"`
	Value   int       `json:"value"`   // 1..10
	Comment string    `json:"comment,omitempty"`
}

// Judgement is one model's full opinion on one (task, output) pair.
type Judgement struct {
	Model    string  `json:"model"`
	OK       bool    `json:"ok"` // false when the model failed to return valid JSON
	Scores   []Score `json:"scores,omitempty"`
	Detail   string  `json:"detail,omitempty"`
}

// Verdict is the aggregate outcome.
type Verdict string

const (
	VerdictPass         Verdict = "pass"
	VerdictFail         Verdict = "fail"
	VerdictInconclusive Verdict = "inconclusive"
)

// Result rolls up Judgement[] into a single decision plus per-dim stats.
type Result struct {
	Verdict     Verdict          `json:"verdict"`
	Reason      string           `json:"reason"`
	OKCount     int              `json:"ok_count"`
	Total       int              `json:"total"`
	Judgements  []Judgement      `json:"judgements"`
	Dimensions  []DimensionStats `json:"dimensions"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// DimensionStats is the per-dim aggregate min/mean/max across OK judges.
type DimensionStats struct {
	Dim   Dimension `json:"dim"`
	Min   int       `json:"min"`
	Max   int       `json:"max"`
	Mean  float64   `json:"mean"`
	Count int       `json:"count"`
}

// MinMean / MinFloor are the gbrain thresholds. Exposed as package vars so
// a future test or operator config can tune them without forking the
// aggregator. **Do not lower these casually**; loosening the gate erases
// the very signal it exists to provide.
var (
	MinMeanScore  = 7.0 // every dim's mean must be >= this
	MinFloorScore = 5   // every dim's min  must be >= this
)

// Aggregate applies the gbrain rules to a slate of judgements and returns
// the rolled-up Result.
func Aggregate(judgements []Judgement) Result {
	r := Result{
		Total:       len(judgements),
		Judgements:  judgements,
		GeneratedAt: time.Now().UTC(),
	}
	for _, j := range judgements {
		if j.OK {
			r.OKCount++
		}
	}

	if r.OKCount < 2 {
		r.Verdict = VerdictInconclusive
		r.Reason = fmt.Sprintf("only %d of %d models returned valid JSON; need at least 2 to vote", r.OKCount, r.Total)
		r.Dimensions = perDimStats(judgements)
		return r
	}

	r.Dimensions = perDimStats(judgements)

	var failures []string
	for _, d := range r.Dimensions {
		if d.Count == 0 {
			failures = append(failures, fmt.Sprintf("%s: no scores", d.Dim))
			continue
		}
		if d.Mean < MinMeanScore {
			failures = append(failures, fmt.Sprintf("%s: mean %.2f < %.1f", d.Dim, d.Mean, MinMeanScore))
		}
		if d.Min < MinFloorScore {
			failures = append(failures, fmt.Sprintf("%s: min %d < %d", d.Dim, d.Min, MinFloorScore))
		}
	}
	if len(failures) == 0 {
		r.Verdict = VerdictPass
		r.Reason = fmt.Sprintf("%d of %d models passed all dimensions", r.OKCount, r.Total)
	} else {
		r.Verdict = VerdictFail
		r.Reason = strings.Join(failures, "; ")
	}
	return r
}

// perDimStats walks every Judgement that returned OK and rolls min/mean/max
// per dimension. A dimension absent from an OK judgement just lowers Count.
func perDimStats(judgements []Judgement) []DimensionStats {
	bucket := make(map[Dimension][]int)
	for _, j := range judgements {
		if !j.OK {
			continue
		}
		for _, s := range j.Scores {
			if s.Value <= 0 {
				continue
			}
			bucket[s.Dim] = append(bucket[s.Dim], s.Value)
		}
	}
	out := make([]DimensionStats, 0, len(AllDimensions))
	for _, d := range AllDimensions {
		vals := bucket[d]
		stats := DimensionStats{Dim: d, Count: len(vals)}
		if len(vals) == 0 {
			out = append(out, stats)
			continue
		}
		sort.Ints(vals)
		stats.Min = vals[0]
		stats.Max = vals[len(vals)-1]
		var sum int
		for _, v := range vals {
			sum += v
		}
		stats.Mean = float64(sum) / float64(len(vals))
		out = append(out, stats)
	}
	return out
}
