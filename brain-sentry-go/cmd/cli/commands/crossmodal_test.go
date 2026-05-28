package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/eval/crossmodal"
)

// --- defaultBuildScorers ---

func TestDefaultBuildScorers_RespectsEnvAndModelFlag(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "sk-or")

	opts := &CrossModalOptions{
		AnthropicModel:  "claude-3-5-sonnet",
		GeminiModel:     "", // disabled (no model flag)
		OpenRouterModel: "openai/gpt-4o-mini",
	}
	scorers, err := defaultBuildScorers(opts)()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(scorers) != 2 {
		t.Errorf("expected 2 scorers (anthropic+openrouter); got %d", len(scorers))
	}
	got := map[string]bool{}
	for _, s := range scorers {
		got[s.Name()] = true
	}
	if !got["anthropic/claude-3-5-sonnet"] || !got["openrouter/openai/gpt-4o-mini"] {
		t.Errorf("expected anthropic+openrouter in scorer set; got %v", got)
	}
}

func TestDefaultBuildScorers_NoKeysReturnsEmpty(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	opts := &CrossModalOptions{
		AnthropicModel:  "claude",
		GeminiModel:     "gemini",
		OpenRouterModel: "gpt",
	}
	scorers, err := defaultBuildScorers(opts)()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(scorers) != 0 {
		t.Errorf("expected 0 scorers without keys; got %d", len(scorers))
	}
}

func TestDefaultBuildScorers_NoModelFlagsReturnsEmpty(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-a")
	t.Setenv("GEMINI_API_KEY", "sk-g")
	t.Setenv("OPENROUTER_API_KEY", "sk-or")
	opts := &CrossModalOptions{} // all model flags empty
	scorers, err := defaultBuildScorers(opts)()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(scorers) != 0 {
		t.Errorf("expected 0 scorers without model flags; got %d", len(scorers))
	}
}

func TestReadTextArg_LiteralAndAtFile(t *testing.T) {
	if got, _ := readTextArg("hello"); got != "hello" {
		t.Errorf("expected literal pass-through; got %q", got)
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "task.txt")
	if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readTextArg("@" + path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "from file" {
		t.Errorf("expected file contents; got %q", got)
	}
}

func TestReadTextArg_AtMissingFileErrors(t *testing.T) {
	_, err := readTextArg("@/no/such/file/here")
	if err == nil {
		t.Errorf("expected error reading missing file")
	}
}

func TestRenderCrossModal_TextHasVerdictAndDimensions(t *testing.T) {
	r := crossmodal.Result{
		Verdict: crossmodal.VerdictFail,
		Reason:  "correctness: mean 5.50 < 7.0",
		OKCount: 2, Total: 3,
		Dimensions: []crossmodal.DimensionStats{
			{Dim: crossmodal.DimCorrectness, Mean: 5.5, Min: 5, Max: 6, Count: 2},
			{Dim: crossmodal.DimSafety, Mean: 9, Min: 9, Max: 9, Count: 2},
		},
		Judgements: []crossmodal.Judgement{
			{Model: "openai/gpt-4o", OK: true},
			{Model: "anthropic/claude", OK: true},
			{Model: "google/gemini", OK: false, Detail: "rate limit"},
		},
	}
	var buf bytes.Buffer
	renderCrossModal(&buf, r)
	out := buf.String()
	for _, want := range []string{"fail", "correctness", "mean=5.50", "[OK] openai", "[FAIL] google", "rate limit"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in render; got %s", want, out)
		}
	}
}
