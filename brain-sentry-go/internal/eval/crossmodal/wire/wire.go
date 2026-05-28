// Package wire bridges internal/service LLM providers into
// internal/eval/crossmodal scorers.
//
// Why a separate package? `internal/eval/crossmodal` is intentionally kept
// free of internal/service imports — service is huge and would balloon
// crossmodal's dependency graph for a tiny adapter. wire/ is the one place
// the two surfaces meet.
package wire

import (
	"context"

	"github.com/integraltech/brainsentry/internal/eval/crossmodal"
	"github.com/integraltech/brainsentry/internal/service"
)

// providerAdapter turns a service.LLMProvider into a crossmodal.ChatProvider.
// Both interfaces are structurally similar; only the ChatMessage struct
// type differs (service.ChatMessage vs crossmodal.ChatMessage).
type providerAdapter struct {
	inner service.LLMProvider
}

func (a *providerAdapter) Name() string { return a.inner.Name() }

func (a *providerAdapter) Chat(ctx context.Context, msgs []crossmodal.ChatMessage) (string, error) {
	out := make([]service.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = service.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return a.inner.Chat(ctx, out)
}

// Scorers builds a list of crossmodal.Scorer from the given service
// providers. Skips nil entries so the operator can pass (anthropic, nil,
// openrouter) without runtime panics when one provider is missing config.
//
// displayNames lets the caller override what shows up in the receipt
// (e.g. "anthropic/claude-3-5" instead of the bare provider name); pass
// empty string for the same index to fall back to provider.Name().
func Scorers(providers []service.LLMProvider, displayNames []string) []crossmodal.Scorer {
	out := make([]crossmodal.Scorer, 0, len(providers))
	for i, p := range providers {
		if p == nil {
			continue
		}
		name := ""
		if i < len(displayNames) {
			name = displayNames[i]
		}
		out = append(out, crossmodal.NewLLMScorer(&providerAdapter{inner: p}, name))
	}
	return out
}
