package crossmodal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeChatProvider struct {
	name string
	seen []ChatMessage
	resp string
	err  error
}

func (f *fakeChatProvider) Name() string { return f.name }
func (f *fakeChatProvider) Chat(_ context.Context, msgs []ChatMessage) (string, error) {
	f.seen = msgs
	return f.resp, f.err
}

func TestLLMScorer_NameOverride(t *testing.T) {
	s := NewLLMScorer(&fakeChatProvider{name: "anthropic"}, "anthropic:claude-3-5")
	if s.Name() != "anthropic:claude-3-5" {
		t.Errorf("expected display name override; got %s", s.Name())
	}
}

func TestLLMScorer_NameFallsBackToProvider(t *testing.T) {
	s := NewLLMScorer(&fakeChatProvider{name: "gemini"}, "")
	if s.Name() != "gemini" {
		t.Errorf("expected fallback to provider name; got %s", s.Name())
	}
}

func TestLLMScorer_ScoreSendsSystemAndUser(t *testing.T) {
	fp := &fakeChatProvider{name: "x", resp: `{"scores":[]}`}
	s := NewLLMScorer(fp, "")
	_, _ = s.Score(context.Background(), "the task", "the output")
	if len(fp.seen) != 2 {
		t.Fatalf("expected 2 messages; got %d", len(fp.seen))
	}
	if fp.seen[0].Role != "system" {
		t.Errorf("expected first message system; got %s", fp.seen[0].Role)
	}
	if !strings.Contains(fp.seen[0].Content, "impartial evaluator") {
		t.Errorf("system prompt missing evaluator framing")
	}
	if !strings.Contains(fp.seen[0].Content, "correctness") {
		t.Errorf("system prompt missing dimension list")
	}
	if fp.seen[1].Role != "user" {
		t.Errorf("expected second message user; got %s", fp.seen[1].Role)
	}
	if !strings.Contains(fp.seen[1].Content, "TASK:\nthe task") {
		t.Errorf("user prompt missing TASK section")
	}
	if !strings.Contains(fp.seen[1].Content, "OUTPUT:\nthe output") {
		t.Errorf("user prompt missing OUTPUT section")
	}
}

func TestLLMScorer_PropagatesProviderError(t *testing.T) {
	fp := &fakeChatProvider{name: "x", err: errors.New("rate limit")}
	s := NewLLMScorer(fp, "")
	_, err := s.Score(context.Background(), "t", "o")
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected error propagated; got %v", err)
	}
}

func TestLLMScorer_NoProviderErrors(t *testing.T) {
	s := &LLMScorer{}
	_, err := s.Score(context.Background(), "t", "o")
	if err == nil || !strings.Contains(err.Error(), "no provider") {
		t.Errorf("expected no-provider error; got %v", err)
	}
}

// End-to-end integration: 3 fake providers with realistic JSON replies
// flow through Run → Aggregate → PASS verdict, mirroring how the wired
// Anthropic + Gemini + OpenAI scorers will behave in production.
func TestLLMScorer_EndToEndPass(t *testing.T) {
	good := `{"scores":[
		{"dim":"correctness","value":9,"comment":"ok"},
		{"dim":"completeness","value":8,"comment":"ok"},
		{"dim":"faithfulness","value":9,"comment":"ok"},
		{"dim":"format","value":9,"comment":"ok"},
		{"dim":"safety","value":10,"comment":"ok"}
	]}`
	scorers := []Scorer{
		NewLLMScorer(&fakeChatProvider{name: "openai", resp: good}, "openai/gpt-4o"),
		NewLLMScorer(&fakeChatProvider{name: "anthropic", resp: good}, "anthropic/claude-3-5"),
		NewLLMScorer(&fakeChatProvider{name: "gemini", resp: good}, "google/gemini-2"),
	}
	res := Run(context.Background(), scorers, "task", "output", time.Second)
	if res.Verdict != VerdictPass {
		t.Errorf("expected pass; got %s (%s)", res.Verdict, res.Reason)
	}
	// Confirm the display names round-tripped into the judgements.
	names := map[string]bool{}
	for _, j := range res.Judgements {
		names[j.Model] = true
	}
	for _, want := range []string{"openai/gpt-4o", "anthropic/claude-3-5", "google/gemini-2"} {
		if !names[want] {
			t.Errorf("expected judge %q present; got %v", want, names)
		}
	}
}

func TestLLMScorer_EndToEndOneVendorDown(t *testing.T) {
	good := `{"scores":[
		{"dim":"correctness","value":9},
		{"dim":"completeness","value":9},
		{"dim":"faithfulness","value":9},
		{"dim":"format","value":9},
		{"dim":"safety","value":9}
	]}`
	scorers := []Scorer{
		NewLLMScorer(&fakeChatProvider{name: "openai", resp: good}, ""),
		NewLLMScorer(&fakeChatProvider{name: "anthropic", resp: good}, ""),
		NewLLMScorer(&fakeChatProvider{name: "gemini", err: errors.New("429 rate limit")}, ""),
	}
	res := Run(context.Background(), scorers, "task", "output", time.Second)
	if res.Verdict != VerdictPass {
		t.Errorf("expected pass with 2/3 voters; got %s (%s)", res.Verdict, res.Reason)
	}
	if res.OKCount != 2 {
		t.Errorf("expected 2 ok; got %d", res.OKCount)
	}
}
