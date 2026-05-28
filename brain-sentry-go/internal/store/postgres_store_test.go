package store

import (
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

func TestToDomain_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	in := MemoryRecord{
		ID: "m1", TenantID: "t1",
		Content: "hello", Summary: "short",
		Category: "INSIGHT", Importance: "CRITICAL",
		Tags:      []string{"a", "b"},
		CreatedAt: now, UpdatedAt: now,
	}
	d := toDomain(in)
	if d.ID != "m1" || d.Content != "hello" {
		t.Errorf("toDomain lost fields: %+v", d)
	}
	if d.Category != domain.MemoryCategory("INSIGHT") {
		t.Errorf("category mis-mapped; got %s", d.Category)
	}
	out := fromDomain(d)
	if out.ID != in.ID || out.Content != in.Content || out.Summary != in.Summary {
		t.Errorf("round-trip lost data: in=%+v out=%+v", in, out)
	}
	if len(out.Tags) != 2 || out.Tags[0] != "a" {
		t.Errorf("tags mis-mapped; got %v", out.Tags)
	}
}

func TestFromDomain_NilSafe(t *testing.T) {
	got := fromDomain(nil)
	if got.ID != "" {
		t.Errorf("expected zero record from nil; got %+v", got)
	}
}

func TestPostgresStore_NilRepoErrors(t *testing.T) {
	s := NewPostgresStore(nil)
	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"create", func() error { _, err := s.Create(nil, MemoryRecord{}); return err }},
		{"get", func() error { _, err := s.Get(nil, "x"); return err }},
		{"list", func() error { _, err := s.List(nil, 5); return err }},
		{"search", func() error { _, err := s.Search(nil, "q", 5); return err }},
		{"delete", func() error { return s.Delete(nil, "x") }},
	} {
		if err := tc.fn(); err == nil {
			t.Errorf("%s: expected error with nil repo", tc.name)
		}
	}
}
