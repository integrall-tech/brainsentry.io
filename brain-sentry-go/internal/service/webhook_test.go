package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
)

func TestWebhookService_RegisterListUnregister(t *testing.T) {
	s := NewWebhookService(nil)
	ctx := context.Background()

	wh := s.Register(ctx, "tenant-a", "http://example.com/hook", "", nil)
	if got := s.ListWebhooks(ctx, "tenant-a"); len(got) != 1 || got[0].ID != wh.ID {
		t.Fatalf("expected registered webhook in list, got %v", got)
	}
	if got := s.ListWebhooks(ctx, "tenant-b"); len(got) != 0 {
		t.Fatalf("expected tenant isolation in list, got %v", got)
	}

	if err := s.Unregister(ctx, wh.ID); err != nil {
		t.Fatalf("unexpected unregister error: %v", err)
	}
	if got := s.ListWebhooks(ctx, "tenant-a"); len(got) != 0 {
		t.Fatalf("expected empty list after unregister, got %v", got)
	}
	if err := s.Unregister(ctx, wh.ID); err == nil {
		t.Fatal("expected error unregistering twice")
	}
}

func TestWebhookService_SuccessResetsFailCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewWebhookService(nil)
	wh := s.Register(context.Background(), "tenant-a", srv.URL, "", nil)
	wh.FailCount = 7
	wh.LastError = "previous failure"

	s.deliver(wh, domain.WebhookEventType("memory.created"), []byte(`{}`))

	s.mu.RLock()
	defer s.mu.RUnlock()
	if wh.FailCount != 0 {
		t.Errorf("expected FailCount reset to 0 after success, got %d", wh.FailCount)
	}
	if wh.LastError != "" {
		t.Errorf("expected LastError cleared after success, got %q", wh.LastError)
	}
}

func TestWebhookService_ConcurrentEmitAndUnregister(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewWebhookService(nil)
	ctx := context.Background()

	// Race Emit against Unregister; run with -race to catch unsynchronized access.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wh := s.Register(ctx, "tenant-a", srv.URL, "secret", nil)
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Emit("tenant-a", domain.WebhookEventType("memory.created"), map[string]string{"k": "v"})
		}()
		go func(id string) {
			defer wg.Done()
			_ = s.Unregister(ctx, id)
		}(wh.ID)
	}
	wg.Wait()
}
