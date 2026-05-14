// Package rebuild implements the in-process executor for `brainsentry
// rebuild`. Each "target" is a derived store (graph, embeddings,
// communities, ...) that can be reconstructed from canonical Postgres.
//
// Design:
//
//   - The Service holds a registry of named Rebuilder closures wired at
//     server startup. cmd/server registers concrete callbacks for the
//     stores it knows about (graph, embeddings, ...). Tests register fakes.
//
//   - The Service has no opinion on *what* a rebuilder does — it just
//     orchestrates the run, captures progress, and aggregates results.
//     This keeps the package free of upward dependencies on every service
//     it might call into.
//
//   - The trust contract: Service.Run() is callable in-process from the
//     CLI (or from a `cmd/server --rebuild=...` startup mode) but is NOT
//     yet exposed over HTTP. Wiring it under a localhost-only HTTP gate
//     is a deliberate next step; for now, in-process keeps the blast
//     radius bounded.
package rebuild

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Rebuilder is the closure a derived store registers. It returns the
// number of artifacts touched (rows truncated, nodes re-inserted, etc.)
// and an error.
//
// Conventions:
//   - Should be idempotent: running twice in a row yields the same end
//     state.
//   - Should respect ctx cancellation; long rebuilds can take minutes.
//   - Counts are advisory — they show up in the report so an operator
//     can sanity-check the work, but no test relies on exact values.
type Rebuilder func(ctx context.Context) (int, error)

// Result is the per-target outcome.
type Result struct {
	Target   string        `json:"target"`
	OK       bool          `json:"ok"`
	Touched  int           `json:"touched"`
	Duration time.Duration `json:"duration_ms"`
	Error    string        `json:"error,omitempty"`
}

// Report aggregates all per-target results.
type Report struct {
	OK       bool          `json:"ok"`
	Duration time.Duration `json:"duration_ms"`
	Results  []Result      `json:"results"`
}

// ErrUnknownTarget is returned by Run when a requested target has no
// registered rebuilder. The message names the unknown target so an
// operator can reconcile against docs/architecture/system-of-record.md.
var ErrUnknownTarget = errors.New("unknown rebuild target")

// Service is the registry of rebuilders.
type Service struct {
	mu         sync.RWMutex
	rebuilders map[string]Rebuilder
}

// New returns an empty Service. Callers wire targets via Register.
func New() *Service {
	return &Service{rebuilders: map[string]Rebuilder{}}
}

// Register binds a rebuilder to a target name. Last-write-wins — the
// caller is expected to register each name only once. Empty names and
// nil callbacks are rejected.
func (s *Service) Register(name string, fn Rebuilder) error {
	if name == "" {
		return errors.New("rebuild: empty target name")
	}
	if fn == nil {
		return errors.New("rebuild: nil rebuilder for " + name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuilders[name] = fn
	return nil
}

// Targets returns the sorted set of registered target names. Stable
// order so CLI / docs comparison is reproducible.
func (s *Service) Targets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.rebuilders))
	for k := range s.rebuilders {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Run executes the named targets serially in the order given. An empty
// `targets` slice runs all registered targets in alphabetical order.
//
// Why serial? Rebuilds touch overlapping resources (graph rebuild reads
// memories; embeddings rebuild writes to the same table). Parallel runs
// would create lock contention with no real wallclock win. If a target
// becomes hot-path enough to need parallelism it can fan out internally.
//
// Run never panics — a panicking Rebuilder is caught and surfaced as a
// failed Result so the operator gets a usable report.
func (s *Service) Run(ctx context.Context, targets []string) Report {
	start := time.Now()
	if len(targets) == 0 {
		targets = s.Targets()
	}
	out := Report{OK: true}
	for _, t := range targets {
		s.mu.RLock()
		fn, ok := s.rebuilders[t]
		s.mu.RUnlock()
		if !ok {
			out.OK = false
			out.Results = append(out.Results, Result{
				Target: t, OK: false,
				Error: fmt.Sprintf("%s: %s", ErrUnknownTarget, t),
			})
			continue
		}
		r := runOne(ctx, t, fn)
		if !r.OK {
			out.OK = false
		}
		out.Results = append(out.Results, r)
	}
	out.Duration = time.Since(start)
	return out
}

func runOne(ctx context.Context, target string, fn Rebuilder) (r Result) {
	r.Target = target
	t0 := time.Now()
	defer func() {
		r.Duration = time.Since(t0)
		if rec := recover(); rec != nil {
			r.OK = false
			r.Error = fmt.Sprintf("panic: %v", rec)
		}
	}()
	n, err := fn(ctx)
	if err != nil {
		r.OK = false
		r.Error = err.Error()
		return
	}
	r.OK = true
	r.Touched = n
	return
}
