package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newAnthropicTestProvider(baseURL string) *AnthropicProvider {
	return NewAnthropicProvider(AnthropicConfig{
		APIKey:      "test-key",
		BaseURL:     baseURL,
		Model:       "claude-test-model",
		MaxTokens:   128,
		Temperature: 0.5,
		Timeout:     5 * time.Second,
	})
}

func TestAnthropicProvider_ChatSuccess(t *testing.T) {
	var gotMethod, gotPath, gotAPIKey, gotVersion, gotContentType string
	var gotBody anthropicRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}`))
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	resp, err := p.Chat(context.Background(), []ChatMessage{
		{Role: "system", Content: "be nice"},
		{Role: "system", Content: "be brief"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hey"},
		{Role: "user", Content: "again"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello world" {
		t.Errorf("expected concatenated text blocks 'hello world', got: %q", resp)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got: %s", gotMethod)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("expected path /v1/messages, got: %s", gotPath)
	}
	if gotAPIKey != "test-key" {
		t.Errorf("expected x-api-key 'test-key', got: %q", gotAPIKey)
	}
	if gotVersion != "2023-06-01" {
		t.Errorf("expected anthropic-version '2023-06-01', got: %q", gotVersion)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got: %q", gotContentType)
	}
	if gotBody.Model != "claude-test-model" {
		t.Errorf("expected model 'claude-test-model', got: %q", gotBody.Model)
	}
	if gotBody.MaxTokens != 128 {
		t.Errorf("expected max_tokens 128, got: %d", gotBody.MaxTokens)
	}
	if gotBody.System != "be nice\n\nbe brief" {
		t.Errorf("expected merged system prompt, got: %q", gotBody.System)
	}
	if len(gotBody.Messages) != 3 {
		t.Fatalf("expected 3 non-system messages, got: %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "user" || gotBody.Messages[0].Content != "hi" {
		t.Errorf("unexpected first message: %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "assistant" || gotBody.Messages[1].Content != "hey" {
		t.Errorf("unexpected second message: %+v", gotBody.Messages[1])
	}
}

func TestAnthropicProvider_HTTPError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"type":"api_error","message":"boom"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestAnthropicProvider_HTTPError429_NoRetry(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, `{"error":{"type":"rate_limit_error","message":"slow down"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on HTTP 429")
	}
	if !strings.Contains(err.Error(), "status 429") {
		t.Errorf("expected error to mention status 429, got: %v", err)
	}
	// The provider has no internal retry; exactly one request must be made.
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly 1 request (no retry), got: %d", got)
	}
}

func TestAnthropicProvider_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on invalid JSON response")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestAnthropicProvider_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"too late"}]}`))
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	start := time.Now()
	_, err := p.Chat(ctx, []ChatMessage{{Role: "user", Content: "hi"}})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if elapsed > time.Second {
		t.Errorf("expected fast failure with cancelled context, took: %v", elapsed)
	}
}

func TestAnthropicProvider_APIErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// status 200 but error object in body
		_, _ = w.Write([]byte(`{"content":[],"error":{"type":"overloaded_error","message":"overloaded"}}`))
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error when body contains error object")
	}
	if !strings.Contains(err.Error(), "overloaded") {
		t.Errorf("expected api error message in error, got: %v", err)
	}
}

func TestAnthropicProvider_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer server.Close()

	p := newAnthropicTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on empty content")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got: %v", err)
	}
}

// Note: Name, RequiresAPIKey and config defaults are covered in llm_providers_test.go.
