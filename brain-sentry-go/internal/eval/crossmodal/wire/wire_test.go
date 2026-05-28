package wire

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/eval/crossmodal"
	"github.com/integraltech/brainsentry/internal/service"
)

type fakeServiceProvider struct {
	name string
	resp string
	seen []service.ChatMessage
}

func (f *fakeServiceProvider) Name() string { return f.name }
func (f *fakeServiceProvider) Chat(_ context.Context, msgs []service.ChatMessage) (string, error) {
	f.seen = msgs
	return f.resp, nil
}

func TestScorers_BridgeRoleContent(t *testing.T) {
	fp := &fakeServiceProvider{name: "anthropic", resp: `{"scores":[]}`}
	scorers := Scorers([]service.LLMProvider{fp}, []string{"anthropic/claude-3-5"})
	if len(scorers) != 1 {
		t.Fatalf("expected 1 scorer; got %d", len(scorers))
	}
	if scorers[0].Name() != "anthropic/claude-3-5" {
		t.Errorf("display name lost; got %s", scorers[0].Name())
	}
	if _, err := scorers[0].Score(context.Background(), "t", "o"); err != nil {
		t.Fatalf("score: %v", err)
	}
	if len(fp.seen) != 2 {
		t.Fatalf("expected 2 messages passed through; got %d", len(fp.seen))
	}
	if fp.seen[0].Role != "system" || !strings.Contains(fp.seen[0].Content, "evaluator") {
		t.Errorf("system prompt mis-mapped; got %+v", fp.seen[0])
	}
	if fp.seen[1].Role != "user" || !strings.Contains(fp.seen[1].Content, "TASK:") {
		t.Errorf("user prompt mis-mapped; got %+v", fp.seen[1])
	}
}

func TestScorers_SkipsNilProviders(t *testing.T) {
	fp := &fakeServiceProvider{name: "x", resp: `{"scores":[]}`}
	scorers := Scorers([]service.LLMProvider{nil, fp, nil}, nil)
	if len(scorers) != 1 {
		t.Errorf("expected nil providers skipped; got %d", len(scorers))
	}
}

func TestScorers_DisplayNameFallsBack(t *testing.T) {
	fp := &fakeServiceProvider{name: "gemini", resp: `{"scores":[]}`}
	scorers := Scorers([]service.LLMProvider{fp}, nil)
	if scorers[0].Name() != "gemini" {
		t.Errorf("expected provider name fallback; got %s", scorers[0].Name())
	}
}

func TestScorers_EndToEndWithRun(t *testing.T) {
	good := `{"scores":[
		{"dim":"correctness","value":9},
		{"dim":"completeness","value":9},
		{"dim":"faithfulness","value":9},
		{"dim":"format","value":9},
		{"dim":"safety","value":9}
	]}`
	scorers := Scorers([]service.LLMProvider{
		&fakeServiceProvider{name: "openai", resp: good},
		&fakeServiceProvider{name: "anthropic", resp: good},
		&fakeServiceProvider{name: "gemini", resp: good},
	}, []string{"openai/gpt-4o-mini", "anthropic/claude-3-5", "google/gemini-2"})
	res := crossmodal.Run(context.Background(), scorers, "task", "output", time.Second)
	if res.Verdict != crossmodal.VerdictPass {
		t.Errorf("expected pass; got %s (%s)", res.Verdict, res.Reason)
	}
}
