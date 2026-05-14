// Package models implements a tier-based model routing system.
//
// The tier abstraction lets calling code ask for a *kind* of model (a fast
// utility, a careful reasoner, a deep researcher, a sub-agent capable of
// using tools) without hard-coding the provider/model ID. Operators tune
// which model fulfils each tier in config or via env vars; calling code is
// insulated from those choices.
//
// Resolution order (highest priority wins):
//
//	1. CLI flag         (--model)
//	2. Per-call override (Resolve(ctx, tier, withOverride("...")))
//	3. Tenant config     (future — not wired here yet)
//	4. Per-tier config   (config.Models.Tier[<tier>])
//	5. Default config    (config.Models.Default)
//	6. Env override      (BRAINSENTRY_MODEL_<TIER> / BRAINSENTRY_MODEL_DEFAULT)
//	7. Built-in tier defaults (TierDefaults map)
//	8. Caller fallback   (Resolve(ctx, tier, withFallback("...")))
//
// Inspired by gbrain's src/core/model-config.ts (8-level resolution chain),
// adapted for Go and brainsentry's OpenRouter-first stack.
package models

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Tier names a class of model use. Add new tiers cautiously — every
// downstream caller now has to decide which tier to ask for.
type Tier string

const (
	// TierUtility — fast, cheap, low-stakes (text classification, query
	// expansion, regex-style routing fallback).
	TierUtility Tier = "utility"
	// TierReasoning — single-shot careful reasoning (compression, fact
	// extraction, schema synthesis).
	TierReasoning Tier = "reasoning"
	// TierDeep — long-context analytical work (cross-session reflection,
	// multi-doc synthesis, ontology drafting).
	TierDeep Tier = "deep"
	// TierSubagent — capable of using tools (only set this to a model that
	// supports the brainsentry MCP / tool-use protocol).
	TierSubagent Tier = "subagent"
)

// AllTiers returns every defined tier in stable order. Used by `models doctor`
// to iterate.
func AllTiers() []Tier {
	return []Tier{TierUtility, TierReasoning, TierDeep, TierSubagent}
}

// Validate ensures the tier is one of the recognized constants.
func (t Tier) Validate() error {
	for _, k := range AllTiers() {
		if k == t {
			return nil
		}
	}
	return fmt.Errorf("unknown tier %q", t)
}

// TierDefaults is the last-resort built-in mapping. Operators *should*
// override this in config; the defaults exist so a fresh checkout can boot
// without env vars set.
var TierDefaults = map[Tier]string{
	TierUtility:   "openai/gpt-4o-mini",
	TierReasoning: "anthropic/claude-3-5-sonnet",
	TierDeep:      "anthropic/claude-opus-4",
	TierSubagent:  "anthropic/claude-3-5-sonnet",
}

// Config is the operator-facing routing config. Lives under config.yaml's
// `models:` key (and is tolerant when absent).
type Config struct {
	Default string          `yaml:"default"`
	Tier    map[Tier]string `yaml:"tier"`
}

// CallOptions controls a single Resolve() call.
type CallOptions struct {
	CLIFlag   string // e.g. value of --model from cobra
	Override  string // per-call override
	Fallback  string // last-resort caller fallback
}

// Option mutates a CallOptions; passed to Resolve.
type Option func(*CallOptions)

// WithCLIFlag sets the CLI-flag value (resolution rank 1).
func WithCLIFlag(model string) Option {
	return func(o *CallOptions) { o.CLIFlag = model }
}

// WithOverride is an explicit per-call override (rank 2).
func WithOverride(model string) Option {
	return func(o *CallOptions) { o.Override = model }
}

// WithFallback is the caller's last-resort if everything else is empty
// (rank 8).
func WithFallback(model string) Option {
	return func(o *CallOptions) { o.Fallback = model }
}

// ResolveResult is the typed outcome of Resolve so callers can log which
// rule they actually got.
type ResolveResult struct {
	Tier   Tier
	Model  string
	Source string // "cli" / "override" / "config-tier" / "config-default" / "env" / "tier-default" / "caller-fallback"
}

// Resolve walks the resolution chain and returns the chosen model + the
// rule that matched. Returns an error only when every rule yields empty.
func Resolve(cfg Config, tier Tier, opts ...Option) (ResolveResult, error) {
	if err := tier.Validate(); err != nil {
		return ResolveResult{}, err
	}
	o := CallOptions{}
	for _, fn := range opts {
		fn(&o)
	}

	// 1. CLI flag
	if v := strings.TrimSpace(o.CLIFlag); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "cli"}, nil
	}
	// 2. Per-call override
	if v := strings.TrimSpace(o.Override); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "override"}, nil
	}
	// 3. tenant config (placeholder — not wired here; future tenants service
	//    will inject via Override).
	// 4. Per-tier config
	if v := strings.TrimSpace(cfg.Tier[tier]); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "config-tier"}, nil
	}
	// 5. Default config
	if v := strings.TrimSpace(cfg.Default); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "config-default"}, nil
	}
	// 6. Env override (BRAINSENTRY_MODEL_<TIER> / _DEFAULT)
	if v := strings.TrimSpace(os.Getenv("BRAINSENTRY_MODEL_" + strings.ToUpper(string(tier)))); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "env"}, nil
	}
	if v := strings.TrimSpace(os.Getenv("BRAINSENTRY_MODEL_DEFAULT")); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "env"}, nil
	}
	// 7. Built-in tier defaults
	if v := TierDefaults[tier]; v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "tier-default"}, nil
	}
	// 8. Caller fallback
	if v := strings.TrimSpace(o.Fallback); v != "" {
		return ResolveResult{Tier: tier, Model: v, Source: "caller-fallback"}, nil
	}
	return ResolveResult{}, errors.New("no model resolved for tier (every rule yielded empty)")
}

// Snapshot returns a deterministic resolution map for every tier — used by
// `brainsentry models` to dump the current routing state.
func Snapshot(cfg Config) []ResolveResult {
	out := make([]ResolveResult, 0, len(AllTiers()))
	for _, t := range AllTiers() {
		r, err := Resolve(cfg, t)
		if err != nil {
			// degrade gracefully — empty model column tells the operator a
			// tier has *no* default at all.
			out = append(out, ResolveResult{Tier: t, Model: "", Source: "(unresolved)"})
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tier < out[j].Tier })
	return out
}
