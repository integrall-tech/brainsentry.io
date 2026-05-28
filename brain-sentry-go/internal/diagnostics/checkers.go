package diagnostics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// --- TCP reachability (composable for any host:port subsystem) ---

// TCPChecker verifies a host:port answers TCP within the timeout.
type TCPChecker struct {
	CheckName string
	Sev       Severity
	Host      string
	Port      int
	Hint      string
}

func (t *TCPChecker) Name() string { return t.CheckName }

func (t *TCPChecker) Check(ctx context.Context) CheckResult {
	addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	dialer := net.Dialer{}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	dctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	conn, err := dialer.DialContext(dctx, "tcp", addr)
	if err != nil {
		return CheckResult{
			Name:     t.CheckName,
			Status:   StatusFail,
			Severity: t.severity(),
			Message:  "TCP dial failed",
			Detail:   fmt.Sprintf("%s: %v", addr, err),
			Hint:     t.Hint,
		}
	}
	_ = conn.Close()
	return CheckResult{
		Name:     t.CheckName,
		Status:   StatusOK,
		Severity: t.severity(),
		Message:  fmt.Sprintf("reachable at %s", addr),
	}
}

func (t *TCPChecker) severity() Severity {
	if t.Sev == "" {
		return SeverityCritical
	}
	return t.Sev
}

// --- HTTP probe (LLM provider, embedding provider, internal /health) ---

// HTTPChecker does a GET (or HEAD) and asserts a 2xx/3xx status.
// If APIKeyHeader is set, it sends the configured key (used as a probe — we
// don't burn a token in this layer; the caller should chain a model-doctor
// for real probe cost).
type HTTPChecker struct {
	CheckName    string
	Sev          Severity
	URL          string
	Method       string // GET / HEAD
	APIKeyHeader string // header name; empty = no auth
	APIKey       string
	Hint         string
	Client       *http.Client
}

func (h *HTTPChecker) Name() string { return h.CheckName }

func (h *HTTPChecker) Check(ctx context.Context) CheckResult {
	if h.URL == "" {
		return CheckResult{
			Name:     h.CheckName,
			Status:   StatusSkip,
			Severity: SeverityInfo,
			Message:  "no URL configured",
		}
	}
	method := h.Method
	if method == "" {
		method = "HEAD"
	}
	req, err := http.NewRequestWithContext(ctx, method, h.URL, nil)
	if err != nil {
		return CheckResult{
			Name: h.CheckName, Status: StatusFail, Severity: h.severity(),
			Message: "bad URL", Detail: err.Error(),
		}
	}
	if h.APIKeyHeader != "" && h.APIKey != "" {
		req.Header.Set(h.APIKeyHeader, h.APIKey)
	}
	cli := h.Client
	if cli == nil {
		cli = &http.Client{Timeout: 4 * time.Second}
	}
	resp, err := cli.Do(req)
	if err != nil {
		return CheckResult{
			Name: h.CheckName, Status: StatusFail, Severity: h.severity(),
			Message: "HTTP probe failed", Detail: err.Error(), Hint: h.Hint,
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return CheckResult{
			Name: h.CheckName, Status: StatusOK, Severity: h.severity(),
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Auth-likely problem — surface differently than a network error.
		return CheckResult{
			Name: h.CheckName, Status: StatusWarn, Severity: SeverityWarning,
			Message: fmt.Sprintf("HTTP %d (auth?)", resp.StatusCode),
			Hint:    "verify API key and provider account is funded",
		}
	}
	return CheckResult{
		Name: h.CheckName, Status: StatusFail, Severity: h.severity(),
		Message: fmt.Sprintf("HTTP %d", resp.StatusCode), Hint: h.Hint,
	}
}

func (h *HTTPChecker) severity() Severity {
	if h.Sev == "" {
		return SeverityCritical
	}
	return h.Sev
}

// --- Generic Func-backed checker for callers that want a closure ---

// FuncChecker wraps an arbitrary closure as a Checker.
type FuncChecker struct {
	CheckName string
	Fn        func(ctx context.Context) CheckResult
}

func (f *FuncChecker) Name() string                       { return f.CheckName }
func (f *FuncChecker) Check(ctx context.Context) CheckResult {
	r := f.Fn(ctx)
	if r.Name == "" {
		r.Name = f.CheckName
	}
	return r
}

// --- Schema version checker (canonical Postgres invariant) ---

// SchemaQueryFn returns the current applied migration version.
type SchemaQueryFn func(ctx context.Context) (string, error)

// SchemaVersionChecker asserts schema is at or above MinVersion.
type SchemaVersionChecker struct {
	CheckName  string
	MinVersion string
	Query      SchemaQueryFn
}

func (s *SchemaVersionChecker) Name() string { return s.CheckName }

func (s *SchemaVersionChecker) Check(ctx context.Context) CheckResult {
	if s.Query == nil {
		return CheckResult{
			Name: s.CheckName, Status: StatusSkip, Severity: SeverityInfo,
			Message: "no schema query configured",
		}
	}
	v, err := s.Query(ctx)
	if err != nil {
		return CheckResult{
			Name: s.CheckName, Status: StatusFail, Severity: SeverityCritical,
			Message: "schema query failed", Detail: err.Error(),
			Hint: "run `brainsentry migrate up`",
		}
	}
	if v < s.MinVersion {
		return CheckResult{
			Name: s.CheckName, Status: StatusFail, Severity: SeverityCritical,
			Message: fmt.Sprintf("schema %s < required %s", v, s.MinVersion),
			Hint:    "run `brainsentry migrate up`",
		}
	}
	return CheckResult{
		Name: s.CheckName, Status: StatusOK, Severity: SeverityCritical,
		Message: fmt.Sprintf("schema at %s", v),
	}
}
