// Package diagnostics provides a "doctor" health & self-check surface that
// validates every external dependency and internal invariant brainsentry.io
// relies on. The same engine powers `brainsentry doctor` (CLI) and
// `GET /v1/diagnostics` (HTTP/admin).
//
// Design goals:
//   - Each checker is independent and side-effect-free (read-only by default).
//   - Output is stable JSON for CI gating and ops dashboards.
//   - A single failed checker yields exit code 1 from the CLI; aggregate is
//     reported either way so the operator can see all problems at once.
//
// Inspired by gbrain's `gbrain doctor` (8 checkers + --fix), adapted to the
// brainsentry.io stack (Postgres, FalkorDB, Redis, LLM provider).
package diagnostics

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status is the outcome of a single checker.
type Status string

const (
	StatusOK     Status = "ok"
	StatusWarn   Status = "warn"
	StatusFail   Status = "fail"
	StatusSkip   Status = "skip"
)

// Severity ranks the importance of a check; aggregate health derives from
// the worst severity that is not StatusOK or StatusSkip.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// CheckResult is the structured output of one Checker.
type CheckResult struct {
	Name     string        `json:"name"`
	Status   Status        `json:"status"`
	Severity Severity      `json:"severity"`
	Message  string        `json:"message"`
	Detail   string        `json:"detail,omitempty"`
	Hint     string        `json:"hint,omitempty"`
	Duration time.Duration `json:"duration_ms"`
}

// Checker probes one subsystem and returns the result.
type Checker interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// Report is the aggregate of all checker outputs.
type Report struct {
	Status     Status        `json:"status"`
	GeneratedAt time.Time    `json:"generated_at"`
	Duration   time.Duration `json:"duration_ms"`
	Checks     []CheckResult `json:"checks"`
	Summary    Summary       `json:"summary"`
}

// Summary tallies counts by status.
type Summary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// Doctor runs a configured set of checkers.
type Doctor struct {
	checkers []Checker
	timeout  time.Duration
}

// New creates a Doctor. timeout is per-checker; 0 means 5s default.
func New(checkers []Checker, timeout time.Duration) *Doctor {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Doctor{checkers: checkers, timeout: timeout}
}

// Run executes every checker (concurrently) and returns the aggregate report.
func (d *Doctor) Run(ctx context.Context) Report {
	start := time.Now()
	results := make([]CheckResult, len(d.checkers))

	var wg sync.WaitGroup
	for i, c := range d.checkers {
		wg.Add(1)
		go func(i int, c Checker) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, d.timeout)
			defer cancel()
			t := time.Now()
			r := c.Check(cctx)
			r.Duration = time.Since(t)
			if r.Name == "" {
				r.Name = c.Name()
			}
			results[i] = r
		}(i, c)
	}
	wg.Wait()

	// Stable order so CI diff stays readable.
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	rep := Report{
		GeneratedAt: time.Now(),
		Duration:    time.Since(start),
		Checks:      results,
	}
	rep.Status, rep.Summary = aggregate(results)
	return rep
}

func aggregate(results []CheckResult) (Status, Summary) {
	var s Summary
	worst := StatusOK
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			s.OK++
		case StatusWarn:
			s.Warn++
			if worst == StatusOK {
				worst = StatusWarn
			}
		case StatusFail:
			s.Fail++
			worst = StatusFail
		case StatusSkip:
			s.Skip++
		}
	}
	return worst, s
}

// FormatText returns a human-friendly multi-line report. CLI uses this when
// --json is not passed.
func (r Report) FormatText() string {
	var sb strings.Builder
	sb.WriteString("brainsentry doctor — ")
	sb.WriteString(string(r.Status))
	sb.WriteString("\n")
	sb.WriteString("checks: ")
	sb.WriteString(itoa(r.Summary.OK))
	sb.WriteString(" ok, ")
	sb.WriteString(itoa(r.Summary.Warn))
	sb.WriteString(" warn, ")
	sb.WriteString(itoa(r.Summary.Fail))
	sb.WriteString(" fail, ")
	sb.WriteString(itoa(r.Summary.Skip))
	sb.WriteString(" skip\n\n")
	for _, c := range r.Checks {
		sb.WriteString("[")
		sb.WriteString(string(c.Status))
		sb.WriteString("] ")
		sb.WriteString(c.Name)
		sb.WriteString(" — ")
		sb.WriteString(c.Message)
		sb.WriteString("\n")
		if c.Detail != "" {
			sb.WriteString("    detail: ")
			sb.WriteString(c.Detail)
			sb.WriteString("\n")
		}
		if c.Hint != "" {
			sb.WriteString("    hint:   ")
			sb.WriteString(c.Hint)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var s []byte
	if n < 0 {
		n = -n
		s = append(s, '-')
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(append(s, digits...))
}

// ExitCode returns 0 for ok / warn-only reports, 1 if any check failed.
// CI gating contract.
func (r Report) ExitCode() int {
	if r.Status == StatusFail {
		return 1
	}
	return 0
}
