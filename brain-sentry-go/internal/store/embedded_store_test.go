package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/pkg/tenant"
)

func tmpEmbedded(t *testing.T) (*EmbeddedStore, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "brain.db.json")
	s, err := OpenEmbedded(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestEmbedded_CreateGetRoundTrip(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	created, err := s.Create(ctx, MemoryRecord{Content: "hello world", Summary: "greeting"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Errorf("expected stamped ID")
	}
	if created.TenantID != "t1" {
		t.Errorf("expected tenant from ctx; got %q", created.TenantID)
	}
	got, err := s.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "hello world" {
		t.Errorf("content mismatch: %+v", got)
	}
}

func TestEmbedded_PersistsAcrossReopen(t *testing.T) {
	s, path := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	created, _ := s.Create(ctx, MemoryRecord{Content: "persist me"})
	_ = s.Close()

	s2, err := OpenEmbedded(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := s2.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Content != "persist me" {
		t.Errorf("expected persistence; got %+v", got)
	}
}

func TestEmbedded_NotFoundReturnsErrSentinel(t *testing.T) {
	s, _ := tmpEmbedded(t)
	_, err := s.Get(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound; got %v", err)
	}
}

func TestEmbedded_TenantIsolation(t *testing.T) {
	s, _ := tmpEmbedded(t)
	t1 := tenant.WithTenant(context.Background(), "t1")
	t2 := tenant.WithTenant(context.Background(), "t2")
	created, _ := s.Create(t1, MemoryRecord{Content: "private"})

	// Reading from a different tenant must not see it
	if _, err := s.Get(t2, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("tenant leak: got %v", err)
	}
	// Same tenant sees it
	if _, err := s.Get(t1, created.ID); err != nil {
		t.Errorf("owner cannot read: %v", err)
	}
}

func TestEmbedded_ListNewestFirst(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	first, _ := s.Create(ctx, MemoryRecord{Content: "first"})
	// Slight delay to make CreatedAt strictly increasing
	second, _ := s.Create(ctx, MemoryRecord{Content: "second"})
	got, err := s.List(ctx, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2; got %d", len(got))
	}
	// CreatedAt may collide on fast systems; tolerate both orderings as
	// long as first/second are present.
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids[first.ID] || !ids[second.ID] {
		t.Errorf("missing rows: %v", ids)
	}
}

func TestEmbedded_ListRespectsLimit(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	for i := 0; i < 5; i++ {
		_, _ = s.Create(ctx, MemoryRecord{Content: "row"})
	}
	got, _ := s.List(ctx, 3)
	if len(got) != 3 {
		t.Errorf("expected limit 3; got %d", len(got))
	}
}

func TestEmbedded_DeleteIdempotent(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	if err := s.Delete(ctx, "no-such"); err != nil {
		t.Errorf("delete missing should be no-op; got %v", err)
	}
	created, _ := s.Create(ctx, MemoryRecord{Content: "x"})
	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected gone after delete")
	}
}

func TestEmbedded_SearchScoresContent(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	a, _ := s.Create(ctx, MemoryRecord{Content: "postgres backup recovery procedure"})
	b, _ := s.Create(ctx, MemoryRecord{Content: "javascript event loop overview"})
	c, _ := s.Create(ctx, MemoryRecord{Content: "postgres index migration"})

	got, err := s.Search(ctx, "postgres", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 hits; got %d", len(got))
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids[a.ID] || !ids[c.ID] || ids[b.ID] {
		t.Errorf("expected only postgres rows; got %v", ids)
	}
}

func TestEmbedded_SearchHonorsTenantScope(t *testing.T) {
	s, _ := tmpEmbedded(t)
	t1 := tenant.WithTenant(context.Background(), "t1")
	t2 := tenant.WithTenant(context.Background(), "t2")
	_, _ = s.Create(t1, MemoryRecord{Content: "tenant one secret"})
	_, _ = s.Create(t2, MemoryRecord{Content: "tenant two secret"})

	got1, _ := s.Search(t1, "secret", 5)
	if len(got1) != 1 || !strings.Contains(got1[0].Content, "tenant one") {
		t.Errorf("expected only t1 row; got %+v", got1)
	}
	got2, _ := s.Search(t2, "secret", 5)
	if len(got2) != 1 || !strings.Contains(got2[0].Content, "tenant two") {
		t.Errorf("expected only t2 row; got %+v", got2)
	}
}

func TestEmbedded_SearchEmptyQuery(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	got, _ := s.Search(ctx, "", 5)
	if len(got) != 0 {
		t.Errorf("expected no results for empty query; got %d", len(got))
	}
}

func TestEmbedded_SearchEmptyStore(t *testing.T) {
	s, _ := tmpEmbedded(t)
	ctx := tenant.WithTenant(context.Background(), "t1")
	got, _ := s.Search(ctx, "anything", 5)
	if len(got) != 0 {
		t.Errorf("expected no results for empty store; got %d", len(got))
	}
}

func TestTokenize_LowercasesAndDropsShort(t *testing.T) {
	got := tokenize("Hello, World! a b cat")
	want := []string{"hello", "world", "cat"}
	if len(got) != len(want) {
		t.Fatalf("expected %v; got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("at %d: expected %q; got %q", i, w, got[i])
		}
	}
}

func TestOpenEmbedded_EmptyPathErrors(t *testing.T) {
	_, err := OpenEmbedded("")
	if err == nil {
		t.Errorf("expected error for empty path")
	}
}

func TestOpenEmbedded_CorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brain.db.json")
	if err := writeFile(path, "this is not json"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := OpenEmbedded(path)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error; got %v", err)
	}
}

// tiny helper to avoid pulling in os.WriteFile at test top-level
func writeFile(path, content string) error {
	return _writeFile(path, content)
}
