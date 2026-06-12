package service

import (
	"context"
	"errors"
	"testing"
)

// Uses mockLLMProvider declared in llm_provider_test.go (same package).

func TestCoreferenceResolve_HappyPath(t *testing.T) {
	llm := &mockLLMProvider{
		name: "mock",
		response: `Here you go:
{"resolved": "Edson deployed the service because Edson owns it.",
 "resolutions": {"he": "Edson", "it": "the service"}}`,
	}
	svc := NewCoreferenceService(llm)

	res, err := svc.Resolve(context.Background(), "He deployed the service because he owns it.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Original != "He deployed the service because he owns it." {
		t.Errorf("original mutated: %q", res.Original)
	}
	if res.Resolved != "Edson deployed the service because Edson owns it." {
		t.Errorf("unexpected resolved text: %q", res.Resolved)
	}
	if len(res.Resolutions) != 2 || res.Resolutions["he"] != "Edson" || res.Resolutions["it"] != "the service" {
		t.Errorf("unexpected resolutions map: %#v", res.Resolutions)
	}
	if llm.callCount != 1 {
		t.Errorf("expected exactly 1 LLM call, got %d", llm.callCount)
	}
}

func TestCoreferenceResolve_EmptyInput(t *testing.T) {
	llm := &mockLLMProvider{name: "mock", response: `{"resolved": "should not be used"}`}
	svc := NewCoreferenceService(llm)

	for _, input := range []string{"", "   ", "\n\t "} {
		res, err := svc.Resolve(context.Background(), input)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if res.Resolved != input || res.Original != input {
			t.Errorf("input %q: expected passthrough, got resolved=%q original=%q", input, res.Resolved, res.Original)
		}
	}
	if llm.callCount != 0 {
		t.Errorf("LLM must not be called for blank input, got %d calls", llm.callCount)
	}
}

func TestCoreferenceResolve_NilLLM(t *testing.T) {
	svc := NewCoreferenceService(nil)

	res, err := svc.Resolve(context.Background(), "She fixed it.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Resolved != "She fixed it." {
		t.Errorf("expected passthrough with nil llm, got %q", res.Resolved)
	}
}

func TestCoreferenceResolve_LLMError(t *testing.T) {
	wantErr := errors.New("provider down")
	svc := NewCoreferenceService(&mockLLMProvider{name: "mock", err: wantErr})

	res, err := svc.Resolve(context.Background(), "He shipped it.")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected provider error to propagate, got %v", err)
	}
	// best-effort: caller still receives the original text unchanged
	if res == nil || res.Resolved != "He shipped it." {
		t.Errorf("expected original text on error, got %#v", res)
	}
}

func TestCoreferenceResolve_MalformedPayload(t *testing.T) {
	cases := map[string]string{
		"no json at all":       "Sorry, I cannot help with that.",
		"unbalanced braces":    "} resolved {",
		"invalid json content": `{"resolved": broken}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			svc := NewCoreferenceService(&mockLLMProvider{name: "mock", response: raw})
			res, err := svc.Resolve(context.Background(), "He did it.")
			if err != nil {
				t.Fatalf("malformed payload must not error: %v", err)
			}
			if res.Resolved != "He did it." {
				t.Errorf("expected passthrough, got %q", res.Resolved)
			}
		})
	}
}

func TestCoreferenceResolve_EmptyResolvedFallsBackToOriginal(t *testing.T) {
	svc := NewCoreferenceService(&mockLLMProvider{
		name:     "mock",
		response: `{"resolved": "", "resolutions": {}}`,
	})

	res, err := svc.Resolve(context.Background(), "They merged the PR.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Resolved != "They merged the PR." {
		t.Errorf("expected fallback to original, got %q", res.Resolved)
	}
}
