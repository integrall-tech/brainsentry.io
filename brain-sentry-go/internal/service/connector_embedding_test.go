package service

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEmbeddingPayload_RoundTrip(t *testing.T) {
	in := embeddingPayload{ChunkID: "doc-1-chunk-0", DocumentID: "doc-1", Content: "hello"}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out embeddingPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestEmbeddingTaskHandler_NoMemoryService(t *testing.T) {
	svc := &ConnectorService{}
	h := svc.EmbeddingTaskHandler()
	payload, _ := json.Marshal(embeddingPayload{ChunkID: "c1", Content: "x"})
	task := &AsyncTask{Type: TaskEmbedding, TenantID: "t1", Payload: payload}
	if err := h(context.Background(), task); err == nil {
		t.Fatal("expected error when memory service is not configured")
	}
}

func TestEmbeddingTaskHandler_MalformedPayload(t *testing.T) {
	svc := (&ConnectorService{}).WithMemoryService(&MemoryService{})
	h := svc.EmbeddingTaskHandler()
	task := &AsyncTask{Type: TaskEmbedding, TenantID: "t1", Payload: json.RawMessage(`{bad`)}
	if err := h(context.Background(), task); err == nil {
		t.Fatal("expected error for malformed payload")
	}
}

// Empty content and missing tenant are dropped (return nil) without touching the
// memory service — so neither path reaches CreateMemory.
func TestEmbeddingTaskHandler_SkipsEmptyAndUntenanted(t *testing.T) {
	svc := (&ConnectorService{}).WithMemoryService(&MemoryService{})
	h := svc.EmbeddingTaskHandler()

	emptyContent, _ := json.Marshal(embeddingPayload{ChunkID: "c1", Content: "   "})
	if err := h(context.Background(), &AsyncTask{Type: TaskEmbedding, TenantID: "t1", Payload: emptyContent}); err != nil {
		t.Errorf("empty content: unexpected error: %v", err)
	}

	noTenant, _ := json.Marshal(embeddingPayload{ChunkID: "c1", Content: "real content"})
	if err := h(context.Background(), &AsyncTask{Type: TaskEmbedding, TenantID: "", Payload: noTenant}); err != nil {
		t.Errorf("missing tenant: unexpected error: %v", err)
	}
}
