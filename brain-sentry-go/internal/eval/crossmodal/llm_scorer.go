package crossmodal

import (
	"context"
	"fmt"
	"strings"
)

// ChatMessage is the role/content pair the LLM scorer feeds into a provider.
// Mirrors service.ChatMessage but defined here so this package has no
// upward dependency on internal/service (which is huge and would force a
// big import surface for a tiny adapter).
type ChatMessage struct {
	Role    string
	Content string
}

// ChatProvider is the minimum a real LLM provider must implement to act as
// a cross-modal scorer. Both AnthropicProvider and GeminiProvider in
// internal/service already match this shape — wiring them is a thin
// adapter in cmd/server/main.go (see NewLLMScorer).
type ChatProvider interface {
	Name() string
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
}

// LLMScorer wraps a ChatProvider as a crossmodal.Scorer. It assembles the
// scoring prompt (system instruction + task + output), calls the provider,
// and returns the raw reply — RepairJSON + ParseJudgement (in scorer.go)
// then coerce that into structured scores.
type LLMScorer struct {
	provider ChatProvider
	// DisplayName lets the operator name the scorer something different
	// from the underlying provider — e.g. "openai" vs "openrouter:gpt-4o"
	// when the same provider proxies multiple vendors. Empty falls back to
	// provider.Name().
	DisplayName string
}

// NewLLMScorer builds a cross-modal Scorer over any chat provider.
func NewLLMScorer(p ChatProvider, displayName string) *LLMScorer {
	return &LLMScorer{provider: p, DisplayName: displayName}
}

// Name returns the operator-friendly identifier surfaced in receipts.
func (s *LLMScorer) Name() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return s.provider.Name()
}

// Score builds the scoring prompt and asks the provider to judge OUTPUT
// against TASK across the 5 dimensions in AllDimensions. Returns the raw
// model reply — the cross-modal aggregator handles repair + parse.
func (s *LLMScorer) Score(ctx context.Context, task, output string) (string, error) {
	if s.provider == nil {
		return "", fmt.Errorf("crossmodal: no provider")
	}
	system := buildScoringSystemPrompt()
	user := buildScoringUserPrompt(task, output)
	reply, err := s.provider.Chat(ctx, []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", err
	}
	return reply, nil
}

// buildScoringSystemPrompt is the instruction every scorer model receives.
// We pin the exact JSON shape and the 1..10 scale so RepairJSON has a
// stable target to coerce models toward. Keep terse — the more we add the
// more brittle the parse becomes when the model paraphrases.
func buildScoringSystemPrompt() string {
	dims := make([]string, 0, len(AllDimensions))
	for _, d := range AllDimensions {
		dims = append(dims, string(d))
	}
	return `You are an impartial evaluator. You will be given a TASK and an OUTPUT.

Score the OUTPUT on each of these dimensions (1 = terrible, 10 = excellent):
  - ` + strings.Join(dims, `
  - `) + `

Respond with ONLY valid JSON, no prose, no code fences:

{"scores":[
  {"dim":"correctness","value":<int 1-10>,"comment":"<one short sentence>"},
  {"dim":"completeness","value":<int 1-10>,"comment":"..."},
  {"dim":"faithfulness","value":<int 1-10>,"comment":"..."},
  {"dim":"format","value":<int 1-10>,"comment":"..."},
  {"dim":"safety","value":<int 1-10>,"comment":"..."}
]}

Be strict. Penalize hallucinations, omissions, structural drift, and unsafe content.`
}

func buildScoringUserPrompt(task, output string) string {
	return "TASK:\n" + task + "\n\nOUTPUT:\n" + output
}
