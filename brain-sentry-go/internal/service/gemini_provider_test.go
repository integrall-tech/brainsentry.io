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

func newGeminiTestProvider(baseURL string) *GeminiProvider {
	return NewGeminiProvider(GeminiConfig{
		APIKey:      "test-key",
		BaseURL:     baseURL,
		Model:       "gemini-test-model",
		MaxTokens:   128,
		Temperature: 0.5,
		Timeout:     5 * time.Second,
	})
}

func TestGeminiProvider_ChatSuccess(t *testing.T) {
	var gotMethod, gotPath, gotKey, gotContentType string
	var gotBody geminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hello "},{"text":"gemini"}]}}]}`))
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
	resp, err := p.Chat(context.Background(), []ChatMessage{
		{Role: "system", Content: "be nice"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hey"},
		{Role: "user", Content: "again"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello gemini" {
		t.Errorf("expected concatenated parts 'hello gemini', got: %q", resp)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got: %s", gotMethod)
	}
	wantPath := "/v1beta/models/gemini-test-model:generateContent"
	if gotPath != wantPath {
		t.Errorf("expected path %s, got: %s", wantPath, gotPath)
	}
	if gotKey != "test-key" {
		t.Errorf("expected key 'test-key' in query, got: %q", gotKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got: %q", gotContentType)
	}
	if gotBody.SystemInstruction == nil {
		t.Fatal("expected systemInstruction to be set")
	}
	if len(gotBody.SystemInstruction.Parts) != 1 || gotBody.SystemInstruction.Parts[0].Text != "be nice" {
		t.Errorf("unexpected systemInstruction: %+v", gotBody.SystemInstruction)
	}
	if len(gotBody.Contents) != 3 {
		t.Fatalf("expected 3 non-system contents, got: %d", len(gotBody.Contents))
	}
	if gotBody.Contents[0].Role != "user" || gotBody.Contents[0].Parts[0].Text != "hi" {
		t.Errorf("unexpected first content: %+v", gotBody.Contents[0])
	}
	if gotBody.Contents[1].Role != "model" || gotBody.Contents[1].Parts[0].Text != "hey" {
		t.Errorf("expected assistant mapped to role 'model', got: %+v", gotBody.Contents[1])
	}
	if gotBody.GenerationConfig == nil {
		t.Fatal("expected generationConfig to be set")
	}
	if gotBody.GenerationConfig.MaxOutputTokens != 128 {
		t.Errorf("expected maxOutputTokens 128, got: %d", gotBody.GenerationConfig.MaxOutputTokens)
	}
	if gotBody.GenerationConfig.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got: %f", gotBody.GenerationConfig.Temperature)
	}
}

func TestGeminiProvider_HTTPError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":500,"message":"internal"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestGeminiProvider_HTTPError429_NoRetry(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, `{"error":{"code":429,"message":"quota exceeded"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
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

func TestGeminiProvider_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`<html>not json</html>`))
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on invalid JSON response")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestGeminiProvider_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"too late"}]}}]}`))
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
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

func TestGeminiProvider_APIErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// status 200 but error object in body
		_, _ = w.Write([]byte(`{"candidates":[],"error":{"code":400,"message":"invalid argument"}}`))
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error when body contains error object")
	}
	if !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("expected api error message in error, got: %v", err)
	}
}

func TestGeminiProvider_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer server.Close()

	p := newGeminiTestProvider(server.URL)
	_, err := p.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("expected error on empty candidates")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got: %v", err)
	}
}

// Note: Name and RequiresAPIKey are covered in llm_providers_test.go.

func TestNewGeminiProvider_Defaults(t *testing.T) {
	p := NewGeminiProvider(GeminiConfig{APIKey: "k"})
	if p.config.BaseURL != "https://generativelanguage.googleapis.com" {
		t.Errorf("expected default BaseURL, got: %s", p.config.BaseURL)
	}
	if p.config.Model == "" {
		t.Error("expected default model to be set")
	}
	if p.config.MaxTokens <= 0 {
		t.Errorf("expected positive default MaxTokens, got: %d", p.config.MaxTokens)
	}
	if p.config.Timeout == 0 {
		t.Error("expected default timeout to be set")
	}
}
