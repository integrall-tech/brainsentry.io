// Package eval implements a capture / export / replay loop for retrieval
// queries — the only honest way to gate retrieval changes (spreading
// activation tweaks, query expansion, source boosting) against real
// traffic.
//
// Inspired by gbrain's `gbrain eval export | replay` (capture wrapper +
// PII-scrubber + Mean Jaccard@k harness).
//
// Workflow:
//
//   1. Operator opts in via BRAINSENTRY_EVAL_CAPTURE=1.
//   2. Every recall/search call gets wrapped: query + top-N memory IDs are
//      written to a Candidate (PII-scrubbed first).
//   3. Operator runs `brainsentry eval export > baseline.ndjson` (stable
//      schema, schema_version 1).
//   4. After a code change, `brainsentry eval replay baseline.ndjson` runs
//      every captured query through the current build and reports:
//        - mean_jaccard@k (intersection / union of top-K id sets)
//        - top1_stability (fraction of queries whose #1 id is unchanged)
//        - latency_delta_ms (median; positive = slower)
//   5. CI gates on `mean_jaccard >= 0.85` (tunable). Failing means the
//      change broke something subtle; passing means it stayed close enough
//      to baseline that the human reviewer can sign off on intentional
//      shifts.
package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// SchemaVersion is the on-wire NDJSON contract. Bump only on
// breaking changes; replay refuses unknown versions so a future-baseline
// can never silently produce nonsense scores.
const SchemaVersion = 1

// EnvFlag is the opt-in env var. Capture is off-by-default to avoid
// inadvertent PII landing in eval logs.
const EnvFlag = "BRAINSENTRY_EVAL_CAPTURE"

// CaptureEnabled reports whether the capture wrapper should record. Read
// via env so the operator can flip it without restart in dev (the wrapper
// re-reads on each call — tiny syscall, tolerable for the quality of
// "no surprise capture" guarantee).
func CaptureEnabled() bool {
	v := os.Getenv(EnvFlag)
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// Candidate is one captured recall. NDJSON-friendly: stable field names,
// pure JSON values, no nested unbounded objects.
type Candidate struct {
	SchemaVersion int       `json:"schema_version"`
	CapturedAt    time.Time `json:"captured_at"`
	TenantID      string    `json:"tenant_id"`
	Query         string    `json:"query"`
	K             int       `json:"k"`
	TopIDs        []string  `json:"top_ids"`
	LatencyMs     int64     `json:"latency_ms"`
	Strategy      string    `json:"strategy,omitempty"` // optional: which retriever picked the top set
}

// Validate checks a candidate is complete enough for replay.
func (c Candidate) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %d not supported (this build expects %d)", c.SchemaVersion, SchemaVersion)
	}
	if c.Query == "" {
		return fmt.Errorf("query is required")
	}
	if c.K <= 0 {
		return fmt.Errorf("k must be > 0")
	}
	return nil
}

// Store buffers candidates in memory until Export. Production deployments
// could swap this for a Postgres-backed implementation; the in-memory store
// is enough to validate the loop and matches gbrain's eval_candidates
// table semantics for small operators.
type Store struct {
	mu  sync.RWMutex
	buf []Candidate
	cap int // ring-buffer cap; 0 = unbounded
}

// NewStore creates a Store. capacity 0 means unlimited (operator knows the
// risk); positive values turn it into a ring buffer.
func NewStore(capacity int) *Store {
	return &Store{cap: capacity}
}

// Add records one candidate after PII scrubbing. Safe to call from many
// goroutines.
func (s *Store) Add(c Candidate) {
	c.SchemaVersion = SchemaVersion
	if c.CapturedAt.IsZero() {
		c.CapturedAt = time.Now().UTC()
	}
	c.Query = ScrubQuery(c.Query)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cap > 0 && len(s.buf) >= s.cap {
		// drop oldest — capture is best-effort; we never want it to become
		// the bottleneck on the hot path.
		copy(s.buf, s.buf[1:])
		s.buf = s.buf[:s.cap-1]
	}
	s.buf = append(s.buf, c)
}

// Snapshot returns a copy of the buffer. The internal buffer is preserved
// so two sequential exports return the same data.
func (s *Store) Snapshot() []Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Candidate, len(s.buf))
	copy(out, s.buf)
	return out
}

// Len returns the current candidate count.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.buf)
}

// Reset empties the buffer. Use after a successful export.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = s.buf[:0]
}

// Export writes the entire buffer to w as NDJSON (one Candidate per line).
// Returns the count written.
func (s *Store) Export(w io.Writer) (int, error) {
	cands := s.Snapshot()
	bw := bufWriter(w)
	enc := json.NewEncoder(bw)
	for _, c := range cands {
		if err := enc.Encode(c); err != nil {
			return 0, err
		}
	}
	if err := flushIfBuffered(bw); err != nil {
		return 0, err
	}
	return len(cands), nil
}

// LoadCandidates parses an NDJSON stream produced by Export. Empty lines
// are tolerated (helps with manual editing).
func LoadCandidates(r io.Reader) ([]Candidate, error) {
	var (
		buf  bytes.Buffer
		cnt  int
		line []byte
		out  []Candidate
	)
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	for _, line = range bytes.Split(buf.Bytes(), []byte{'\n'}) {
		cnt++
		t := bytes.TrimSpace(line)
		if len(t) == 0 {
			continue
		}
		var c Candidate
		if err := json.Unmarshal(t, &c); err != nil {
			return nil, fmt.Errorf("line %d: %w", cnt, err)
		}
		if err := c.Validate(); err != nil {
			return nil, fmt.Errorf("line %d: %w", cnt, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// --- buffered writer helpers without allocating wrappers when we don't need to ---

type flusher interface{ Flush() error }

func bufWriter(w io.Writer) io.Writer {
	if _, ok := w.(flusher); ok {
		return w
	}
	return w
}

func flushIfBuffered(w io.Writer) error {
	if f, ok := w.(flusher); ok {
		return f.Flush()
	}
	return nil
}
