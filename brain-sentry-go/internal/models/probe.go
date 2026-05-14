package models

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FailureKind classifies a probe error into a category an operator can act
// on. Mirrors gbrain's `models doctor` taxonomy.
type FailureKind string

const (
	FailureNone           FailureKind = ""
	FailureModelNotFound  FailureKind = "model_not_found"
	FailureAuth           FailureKind = "auth"
	FailureRateLimit      FailureKind = "rate_limit"
	FailureNetwork        FailureKind = "network"
	FailureTimeout        FailureKind = "timeout"
	FailureInvalidRequest FailureKind = "invalid_request"
	FailureUnknown        FailureKind = "unknown"
)

// ProbeResult is the structured outcome of one probe attempt.
type ProbeResult struct {
	Tier     Tier          `json:"tier"`
	Model    string        `json:"model"`
	OK       bool          `json:"ok"`
	Failure  FailureKind   `json:"failure,omitempty"`
	Duration time.Duration `json:"duration_ms"`
	Detail   string        `json:"detail,omitempty"`
	Hint     string        `json:"hint,omitempty"`
}

// Prober runs the actual 1-token call. Defined as an interface so tests can
// inject deterministic fakes and so future providers slot in without
// changing the doctor.
type Prober interface {
	Probe(ctx context.Context, model string) error
}

// DoctorReport aggregates probe results across every tier.
type DoctorReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Duration    time.Duration `json:"duration_ms"`
	OK          bool          `json:"ok"`
	Results     []ProbeResult `json:"results"`
}

// RunDoctor probes every tier sequentially (parallel would burn rate limit
// for no real benefit on a 4-element list). Returns OK=true only when every
// probe passed.
func RunDoctor(ctx context.Context, cfg Config, prober Prober, perCallTimeout time.Duration) DoctorReport {
	if perCallTimeout <= 0 {
		perCallTimeout = 8 * time.Second
	}
	start := time.Now()
	rep := DoctorReport{GeneratedAt: start, OK: true}
	for _, t := range AllTiers() {
		res := probeTier(ctx, cfg, prober, t, perCallTimeout)
		if !res.OK {
			rep.OK = false
		}
		rep.Results = append(rep.Results, res)
	}
	rep.Duration = time.Since(start)
	return rep
}

func probeTier(ctx context.Context, cfg Config, prober Prober, t Tier, perCallTimeout time.Duration) ProbeResult {
	resolved, err := Resolve(cfg, t)
	if err != nil {
		return ProbeResult{
			Tier: t, OK: false, Failure: FailureInvalidRequest,
			Detail: err.Error(),
			Hint:   "set models.tier." + string(t) + " in config.yaml or BRAINSENTRY_MODEL_" + strings.ToUpper(string(t)),
		}
	}
	cctx, cancel := context.WithTimeout(ctx, perCallTimeout)
	defer cancel()

	t0 := time.Now()
	if prober == nil {
		return ProbeResult{
			Tier: t, Model: resolved.Model, OK: false, Failure: FailureUnknown,
			Detail: "no prober wired",
			Hint:   "ensure brainsentry was started with an LLM provider configured",
		}
	}
	err = prober.Probe(cctx, resolved.Model)
	dur := time.Since(t0)

	if err == nil {
		return ProbeResult{Tier: t, Model: resolved.Model, OK: true, Duration: dur}
	}
	kind, hint := classify(err)
	return ProbeResult{
		Tier: t, Model: resolved.Model, OK: false,
		Failure: kind, Duration: dur, Detail: err.Error(), Hint: hint,
	}
}

// classify converts a transport error into one of the canonical failure
// kinds. Best-effort string matching — providers don't standardize their
// error envelopes, but the same words show up across them.
func classify(err error) (FailureKind, string) {
	if err == nil {
		return FailureNone, ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return FailureTimeout, "increase --timeout or check provider latency"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "404") || strings.Contains(msg, "not found") || strings.Contains(msg, "no such model") || strings.Contains(msg, "unknown model"):
		return FailureModelNotFound, "the model id does not exist on the provider — check for typos / phantom IDs"
	case strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid api key") || strings.Contains(msg, "authentication"):
		return FailureAuth, "verify the API key and that it grants access to this model"
	case strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "rate-limit") || strings.Contains(msg, "too many"):
		return FailureRateLimit, "wait or move this tier to a less-saturated provider"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") || strings.Contains(msg, "i/o timeout"):
		return FailureNetwork, "verify network connectivity and provider base URL"
	case strings.Contains(msg, "400") || strings.Contains(msg, "invalid request"):
		return FailureInvalidRequest, "the request shape is wrong — usually means the model id maps to a different schema (e.g. completions vs chat)"
	}
	return FailureUnknown, "open-ended provider error — see Detail"
}

// HTTPProber is a generic 1-token probe against any OpenAI/OpenRouter-compat
// chat completions endpoint. Most providers we care about respect this
// shape; one config knob (BaseURL + APIKey) covers OpenRouter, Anthropic
// (via OpenAI-compat), and self-hosted Ollama.
//
// Wraps a *http.Client with sensible timeouts; uses POST with a tiny body
// so the probe burns a single token (or fewer with most providers).
type HTTPProber struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
	BuildBody func(model string) (string, string) // returns (path, body); allows per-provider customization
}

// Probe issues a 1-token chat completion against BaseURL.
func (p *HTTPProber) Probe(ctx context.Context, model string) error {
	if p.Client == nil {
		p.Client = &http.Client{Timeout: 10 * time.Second}
	}
	build := p.BuildBody
	if build == nil {
		build = defaultProbeBody
	}
	path, body := build(model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

// defaultProbeBody is the OpenAI-style chat-completions request that costs a
// single token in most providers. Body is hand-rolled to skip an encoder
// allocation on the hot health-check path.
func defaultProbeBody(model string) (string, string) {
	return "/chat/completions",
		`{"model":` + jsonString(model) + `,"max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`
}

func jsonString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
