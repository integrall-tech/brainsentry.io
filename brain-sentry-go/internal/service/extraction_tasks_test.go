package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/integraltech/brainsentry/pkg/tenant"
)

func TestExtractionPayload_RoundTrip(t *testing.T) {
	in := extractionPayload{MemoryID: "mem-1", Content: "Alice deploys service X"}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out extractionPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestExtractionTaskHandler_MalformedPayload(t *testing.T) {
	svc := &MemoryService{}
	h := svc.ExtractionTaskHandler()
	task := &AsyncTask{Type: TaskTripletExtraction, Payload: json.RawMessage(`{not json`)}
	if err := h(context.Background(), task); err == nil {
		t.Fatal("expected error for malformed payload, got nil")
	}
}

// With no extractor services configured, the handler must complete cleanly:
// runExtraction guards on nil svc and returns nil. This exercises the full glue
// (unmarshal -> tenant context -> dispatch by task type) without an LLM.
func TestExtractionTaskHandler_NilServicesNoop(t *testing.T) {
	svc := &MemoryService{}
	h := svc.ExtractionTaskHandler()
	payload, _ := json.Marshal(extractionPayload{MemoryID: "mem-1", Content: "x"})

	for _, tt := range []TaskType{TaskTripletExtraction, TaskEventExtraction} {
		task := &AsyncTask{Type: tt, TenantID: "tenant-a", Payload: payload}
		if err := h(context.Background(), task); err != nil {
			t.Errorf("type %s: unexpected error: %v", tt, err)
		}
	}
}

func TestRunExtraction_UnknownType(t *testing.T) {
	svc := &MemoryService{}
	if err := svc.runExtraction(context.Background(), "bogus_type", "mem-1", "x"); err == nil {
		t.Fatal("expected error for unknown task type, got nil")
	}
}

// With a scheduler configured, dispatchExtraction must enqueue a task carrying
// the tenant and the memory payload. A scheduler built with a nil Redis client
// processes inline (processTaskInline), so we observe the delivered task in the
// registered handler — exercising the real dispatch -> Submit -> handler path
// without Redis and without any test-only seam in production code.
func TestDispatchExtraction_SchedulerReceivesTaskWithTenantAndPayload(t *testing.T) {
	sched := NewTaskScheduler(nil, DefaultTaskSchedulerConfig())
	got := make(chan *AsyncTask, 1)
	sched.RegisterHandler(TaskTripletExtraction, func(ctx context.Context, task *AsyncTask) error {
		_ = tenant.FromContext(ctx) // handler is responsible for reconstructing tenant
		got <- task
		return nil
	})

	svc := (&MemoryService{}).WithTaskScheduler(sched)
	svc.dispatchExtraction(context.Background(), TaskTripletExtraction, "tenant-z", "mem-1", "hello world")

	task := <-got
	if task.TenantID != "tenant-z" {
		t.Errorf("task.TenantID = %q, want %q", task.TenantID, "tenant-z")
	}
	if task.Type != TaskTripletExtraction {
		t.Errorf("task.Type = %q, want %q", task.Type, TaskTripletExtraction)
	}
	var p extractionPayload
	if err := json.Unmarshal(task.Payload, &p); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if p.MemoryID != "mem-1" || p.Content != "hello world" {
		t.Errorf("payload = %+v, want {mem-1, hello world}", p)
	}
}
