package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/eval/crossmodal"
)

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
