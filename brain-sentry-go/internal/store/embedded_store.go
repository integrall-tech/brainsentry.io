package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// EmbeddedStore is a zero-config, single-file backend. The whole memory
// table is loaded into a map on Open and persisted as JSON on every write.
// File locking is process-local (sync.Mutex) — fine for a single-process
// developer setup, not appropriate for production with multiple writers.
//
// Why JSON instead of SQLite or BoltDB?
//   - Zero CGO, zero binary dependency, ~200 LoC.
//   - The audience is "developer who cloned the repo and wants something
//     working in 30 seconds." For that audience the perf cost of linear
//     scan + JSON re-serialize on write is irrelevant.
//   - When this proves insufficient, the MemoryStore interface lets us
//     swap in a real SQLite without touching callers.
type EmbeddedStore struct {
	path string
	mu   sync.RWMutex
	rows map[string]MemoryRecord // keyed by ID
}

// OpenEmbedded loads (or creates) the JSON-backed store at path. A non-
// existent file is treated as empty; the directory is created if missing.
func OpenEmbedded(path string) (*EmbeddedStore, error) {
	if path == "" {
		return nil, errors.New("path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	s := &EmbeddedStore{path: path, rows: map[string]MemoryRecord{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Materialize an empty file so subsequent inspection can confirm
			// the workspace exists before any memories are written.
			_ = os.WriteFile(path, []byte("[]"), 0o600)
			return s, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(b) == 0 {
		// Materialize an empty file so subsequent inspection (`ls`, doctor)
		// can confirm the store exists before any memories are written.
		_ = os.WriteFile(path, []byte("[]"), 0o600)
		return s, nil
	}
	var rows []MemoryRecord
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, r := range rows {
		s.rows[r.ID] = r
	}
	return s, nil
}

// Create persists a new memory. Stamps ID + CreatedAt/UpdatedAt when
// missing.
func (s *EmbeddedStore) Create(ctx context.Context, m MemoryRecord) (MemoryRecord, error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.TenantID == "" {
		m.TenantID = tenant.FromContext(ctx)
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[m.ID] = m
	if err := s.flushLocked(); err != nil {
		return MemoryRecord{}, err
	}
	return m, nil
}

// Get returns a memory by ID, ensuring tenant scoping.
func (s *EmbeddedStore) Get(ctx context.Context, id string) (MemoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rows[id]
	if !ok {
		return MemoryRecord{}, ErrNotFound
	}
	if !sameTenant(ctx, r) {
		return MemoryRecord{}, ErrNotFound
	}
	return r, nil
}

// List returns memories for the tenant ordered by CreatedAt DESC.
func (s *EmbeddedStore) List(ctx context.Context, limit int) ([]MemoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MemoryRecord, 0, len(s.rows))
	for _, r := range s.rows {
		if sameTenant(ctx, r) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Search ranks memories with a tiny bag-of-words TF-IDF-ish score: each
// query term contributes 1/log(1+df) to the document's score for every
// occurrence in (Content + Summary). Far from production-grade IR but
// good enough to *route* a query to the right handful of memories on
// small datasets, which is exactly the embedded-mode use case.
func (s *EmbeddedStore) Search(ctx context.Context, query string, limit int) ([]MemoryRecord, error) {
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Compute df per term scoped to tenant.
	df := make(map[string]int, len(terms))
	type docTokens struct {
		rec    MemoryRecord
		tokens []string
	}
	docs := make([]docTokens, 0, len(s.rows))
	for _, r := range s.rows {
		if !sameTenant(ctx, r) {
			continue
		}
		toks := tokenize(r.Content + " " + r.Summary)
		docs = append(docs, docTokens{rec: r, tokens: toks})
		seen := map[string]bool{}
		for _, t := range toks {
			if !seen[t] {
				seen[t] = true
				if termIn(t, terms) {
					df[t]++
				}
			}
		}
	}

	type scored struct {
		rec   MemoryRecord
		score float64
	}
	results := make([]scored, 0, len(docs))
	for _, d := range docs {
		var score float64
		for _, t := range terms {
			tf := 0
			for _, dt := range d.tokens {
				if dt == t {
					tf++
				}
			}
			if tf == 0 {
				continue
			}
			idf := 1.0 / math.Log(1+float64(df[t]))
			score += float64(tf) * idf
		}
		if score > 0 {
			results = append(results, scored{rec: d.rec, score: score})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	out := make([]MemoryRecord, len(results))
	for i, r := range results {
		out[i] = r.rec
	}
	return out, nil
}

// Delete removes by ID. Idempotent.
func (s *EmbeddedStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[id]; !ok {
		return nil
	}
	delete(s.rows, id)
	return s.flushLocked()
}

// Close is a no-op for the embedded store; flushes are synchronous on
// every write so there is no buffered state to drain.
func (s *EmbeddedStore) Close() error { return nil }

// flushLocked writes the row map to disk as a stable-ordered JSON array.
// Caller must hold s.mu (write).
func (s *EmbeddedStore) flushLocked() error {
	rows := make([]MemoryRecord, 0, len(s.rows))
	for _, r := range s.rows {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID }) // deterministic file
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func sameTenant(ctx context.Context, r MemoryRecord) bool {
	t := tenant.FromContext(ctx)
	return r.TenantID == "" || r.TenantID == t
}

// tokenize lowercases and splits on non-alphanumeric runes; drops tokens
// shorter than 3 chars to filter out noise.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	out := []string{}
	curr := strings.Builder{}
	flush := func() {
		if curr.Len() >= 3 {
			out = append(out, curr.String())
		}
		curr.Reset()
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			curr.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func termIn(token string, terms []string) bool {
	for _, t := range terms {
		if t == token {
			return true
		}
	}
	return false
}
