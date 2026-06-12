package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockLLMProvider implements LLMProvider for testing.
type mockLLMProvider struct {
	name      string
	response  string
	err       error
	callCount int
}

func (m *mockLLMProvider) Name() string { return m.name }

func (m *mockLLMProvider) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestFallbackChainProvider_FirstSucceeds(t *testing.T) {
	p1 := &mockLLMProvider{name: "primary", response: "hello from primary"}
	p2 := &mockLLMProvider{name: "secondary", response: "hello from secondary"}

	chain := NewFallbackChainProvider(p1, p2)
	resp, err := chain.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "test"}})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello from primary" {
		t.Errorf("expected primary response, got: %s", resp)
	}
	if p1.callCount != 1 {
		t.Errorf("expected primary called once, got %d", p1.callCount)
	}
	if p2.callCount != 0 {
		t.Errorf("expected secondary not called, got %d", p2.callCount)
	}
}

func TestFallbackChainProvider_FallsBackToSecond(t *testing.T) {
	p1 := &mockLLMProvider{name: "primary", err: errors.New("provider down")}
	p2 := &mockLLMProvider{name: "secondary", response: "hello from secondary"}

	chain := NewFallbackChainProvider(p1, p2)
	resp, err := chain.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "test"}})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello from secondary" {
		t.Errorf("expected secondary response, got: %s", resp)
	}
	if p1.callCount != 1 {
		t.Errorf("expected primary called once, got %d", p1.callCount)
	}
	if p2.callCount != 1 {
		t.Errorf("expected secondary called once, got %d", p2.callCount)
	}
}

func TestFallbackChainProvider_AllFail(t *testing.T) {
	p1 := &mockLLMProvider{name: "primary", err: errors.New("down")}
	p2 := &mockLLMProvider{name: "secondary", err: errors.New("also down")}

	chain := NewFallbackChainProvider(p1, p2)
	_, err := chain.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "test"}})

	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestFallbackChainProvider_Name(t *testing.T) {
	chain := NewFallbackChainProvider()
	if chain.Name() != "fallback-chain" {
		t.Errorf("expected name 'fallback-chain', got: %s", chain.Name())
	}
}

func TestFallbackChainProvider_ProviderStatus(t *testing.T) {
	p1 := &mockLLMProvider{name: "primary", response: "ok"}
	chain := NewFallbackChainProvider(p1)

	status := chain.ProviderStatus()
	if status["primary"] != "CLOSED" {
		t.Errorf("expected CLOSED, got: %s", status["primary"])
	}
}

// ctxAwareLLMProvider blocks until the context is done, then returns ctx.Err().
// Simulates a slow backend that honors cancellation.
type ctxAwareLLMProvider struct {
	name      string
	callCount int
}

func (m *ctxAwareLLMProvider) Name() string { return m.name }

func (m *ctxAwareLLMProvider) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	m.callCount++
	<-ctx.Done()
	return "", ctx.Err()
}

func TestFallbackChainProvider_ContextCancelled(t *testing.T) {
	p1 := &ctxAwareLLMProvider{name: "primary"}
	p2 := &ctxAwareLLMProvider{name: "secondary"}
	chain := NewFallbackChainProvider(p1, p2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	start := time.Now()
	_, err := chain.Chat(ctx, []ChatMessage{{Role: "user", Content: "test"}})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected error to wrap context.Canceled, got: %v", err)
	}
	if elapsed > time.Second {
		t.Errorf("expected fast failure with cancelled context, took: %v", elapsed)
	}
}

func TestFallbackChainProvider_TimeoutRespected(t *testing.T) {
	p1 := &ctxAwareLLMProvider{name: "slow-primary"}
	chain := NewFallbackChainProvider(p1)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := chain.Chat(ctx, []ChatMessage{{Role: "user", Content: "test"}})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when provider exceeds context deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected error to wrap context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected return shortly after 100ms deadline, took: %v", elapsed)
	}
	if p1.callCount != 1 {
		t.Errorf("expected provider called once, got: %d", p1.callCount)
	}
}

func TestFallbackChainProvider_CircuitOpensAndSkipsProvider(t *testing.T) {
	p1 := &mockLLMProvider{name: "flaky", err: errors.New("down")}
	p2 := &mockLLMProvider{name: "stable", response: "ok"}
	chain := NewFallbackChainProvider(p1, p2)

	// FailureThreshold is 3 in NewFallbackChainProvider; trip the breaker.
	for i := 0; i < 3; i++ {
		resp, err := chain.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "test"}})
		if err != nil {
			t.Fatalf("call %d: unexpected error (fallback should succeed): %v", i, err)
		}
		if resp != "ok" {
			t.Fatalf("call %d: expected fallback response 'ok', got: %q", i, resp)
		}
	}

	status := chain.ProviderStatus()
	if status["flaky"] != "OPEN" {
		t.Errorf("expected flaky breaker OPEN after 3 failures, got: %s", status["flaky"])
	}
	if status["stable"] != "CLOSED" {
		t.Errorf("expected stable breaker CLOSED, got: %s", status["stable"])
	}

	// With the breaker open, the flaky provider must be skipped entirely.
	callsBefore := p1.callCount
	resp, err := chain.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected fallback response 'ok', got: %q", resp)
	}
	if p1.callCount != callsBefore {
		t.Errorf("expected flaky provider skipped while breaker OPEN, but call count went %d -> %d", callsBefore, p1.callCount)
	}
}

func TestOpenRouterProvider_Name(t *testing.T) {
	p := NewOpenRouterProvider(&OpenRouterService{})
	if p.Name() != "openrouter" {
		t.Errorf("expected 'openrouter', got: %s", p.Name())
	}
}
