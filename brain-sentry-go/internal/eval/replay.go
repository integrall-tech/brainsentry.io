package eval

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// ReplayResult is the per-query outcome of replaying one Candidate against
// the current build.
type ReplayResult struct {
	Query        string   `json:"query"`
	K            int      `json:"k"`
	BaselineIDs  []string `json:"baseline_ids"`
	CurrentIDs   []string `json:"current_ids"`
	Jaccard      float64  `json:"jaccard"`
	Top1Stable   bool     `json:"top1_stable"`
	BaselineMs   int64    `json:"baseline_latency_ms"`
	CurrentMs    int64    `json:"current_latency_ms"`
	LatencyDelta int64    `json:"latency_delta_ms"`
	Error        string   `json:"error,omitempty"`
}

// ReplaySummary aggregates results across many queries.
type ReplaySummary struct {
	Total           int           `json:"total"`
	Failed          int           `json:"failed"`
	MeanJaccard     float64       `json:"mean_jaccard"`
	Top1Stability   float64       `json:"top1_stability"`
	MedianLatencyDelta int64      `json:"median_latency_delta_ms"`
	Duration        time.Duration `json:"duration_ms"`
	Results         []ReplayResult `json:"results"`
}

// Pass returns the boolean a CI gate cares about: aggregate jaccard above
// the threshold and no per-query failures.
func (s ReplaySummary) Pass(minJaccard float64) bool {
	return s.Failed == 0 && s.MeanJaccard >= minJaccard
}

// QueryFn runs one recall against the current build and returns the top-K
// memory IDs (in rank order) and how long it took.
type QueryFn func(ctx context.Context, query string, k int) ([]string, time.Duration, error)

// Run replays every candidate and aggregates the score.
func Run(ctx context.Context, baseline []Candidate, fn QueryFn) ReplaySummary {
	start := time.Now()
	sum := ReplaySummary{Total: len(baseline)}

	deltas := make([]int64, 0, len(baseline))
	for _, c := range baseline {
		r := ReplayResult{
			Query:       c.Query,
			K:           c.K,
			BaselineIDs: c.TopIDs,
			BaselineMs:  c.LatencyMs,
		}
		current, dur, err := fn(ctx, c.Query, c.K)
		if err != nil {
			r.Error = err.Error()
			sum.Failed++
			sum.Results = append(sum.Results, r)
			continue
		}
		r.CurrentIDs = current
		r.CurrentMs = dur.Milliseconds()
		r.LatencyDelta = r.CurrentMs - r.BaselineMs
		r.Jaccard = Jaccard(c.TopIDs, current)
		r.Top1Stable = top1Equal(c.TopIDs, current)

		sum.MeanJaccard += r.Jaccard
		if r.Top1Stable {
			sum.Top1Stability += 1
		}
		deltas = append(deltas, r.LatencyDelta)
		sum.Results = append(sum.Results, r)
	}

	if scored := sum.Total - sum.Failed; scored > 0 {
		sum.MeanJaccard /= float64(scored)
		sum.Top1Stability /= float64(scored)
	}
	sum.MedianLatencyDelta = median(deltas)
	sum.Duration = time.Since(start)
	return sum
}

// Jaccard computes |a ∩ b| / |a ∪ b| over two id sets. Returns 1 when both
// are empty (vacuously identical) and 0 when one is empty and the other is
// not.
func Jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	set := make(map[string]int, len(a)+len(b))
	for _, x := range a {
		set[x] |= 1
	}
	for _, x := range b {
		set[x] |= 2
	}
	var inter, union int
	for _, mask := range set {
		union++
		if mask == 3 {
			inter++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func top1Equal(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return a[0] == b[0]
}

func median(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]int64, len(xs))
	copy(cp, xs)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

// FormatText writes a human-friendly replay report.
func (s ReplaySummary) FormatText() string {
	verdict := "PASS"
	if s.Failed > 0 {
		verdict = "FAIL"
	}
	out := fmt.Sprintf("brainsentry eval replay — %s\n", verdict)
	out += fmt.Sprintf("  total:        %d\n", s.Total)
	out += fmt.Sprintf("  failed:       %d\n", s.Failed)
	out += fmt.Sprintf("  mean jaccard: %.3f\n", s.MeanJaccard)
	out += fmt.Sprintf("  top-1 stable: %.1f%%\n", s.Top1Stability*100)
	out += fmt.Sprintf("  median Δ ms:  %+d\n", s.MedianLatencyDelta)
	out += fmt.Sprintf("  duration:     %dms\n", s.Duration.Milliseconds())
	return out
}
