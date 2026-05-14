package rebuild

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegister_RejectsEmptyOrNil(t *testing.T) {
	s := New()
	if err := s.Register("", func(_ context.Context) (int, error) { return 0, nil }); err == nil {
		t.Errorf("expected error for empty name")
	}
	if err := s.Register("graph", nil); err == nil {
		t.Errorf("expected error for nil rebuilder")
	}
}

func TestTargets_SortedDeduped(t *testing.T) {
	s := New()
	_ = s.Register("graph", func(_ context.Context) (int, error) { return 0, nil })
	_ = s.Register("embeddings", func(_ context.Context) (int, error) { return 0, nil })
	_ = s.Register("communities", func(_ context.Context) (int, error) { return 0, nil })
	got := s.Targets()
	want := []string{"communities", "embeddings", "graph"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets; got %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("at %d: expected %s; got %s", i, w, got[i])
		}
	}
}

func TestRun_AllRegisteredAlphabetical(t *testing.T) {
	s := New()
	var order []string
	make := func(name string) Rebuilder {
		return func(_ context.Context) (int, error) {
			order = append(order, name)
			return 1, nil
		}
	}
	_ = s.Register("graph", make("graph"))
	_ = s.Register("embeddings", make("embeddings"))
	_ = s.Register("communities", make("communities"))

	rep := s.Run(context.Background(), nil)
	if !rep.OK {
		t.Fatalf("expected ok aggregate; got %+v", rep)
	}
	if strings.Join(order, ",") != "communities,embeddings,graph" {
		t.Errorf("expected alphabetical order; got %v", order)
	}
}

func TestRun_RespectsRequestedOrder(t *testing.T) {
	s := New()
	var order []string
	make := func(name string) Rebuilder {
		return func(_ context.Context) (int, error) {
			order = append(order, name)
			return 0, nil
		}
	}
	_ = s.Register("a", make("a"))
	_ = s.Register("b", make("b"))
	_ = s.Register("c", make("c"))
	_ = s.Run(context.Background(), []string{"c", "a", "b"})
	if strings.Join(order, ",") != "c,a,b" {
		t.Errorf("expected requested order; got %v", order)
	}
}

func TestRun_UnknownTargetMarksFailure(t *testing.T) {
	s := New()
	rep := s.Run(context.Background(), []string{"nope"})
	if rep.OK {
		t.Errorf("expected ok=false")
	}
	if len(rep.Results) != 1 {
		t.Fatalf("expected 1 result; got %d", len(rep.Results))
	}
	if !strings.Contains(rep.Results[0].Error, "unknown rebuild target") {
		t.Errorf("expected unknown-target error; got %q", rep.Results[0].Error)
	}
}

func TestRun_RebuilderErrorRecorded(t *testing.T) {
	s := New()
	_ = s.Register("graph", func(_ context.Context) (int, error) {
		return 0, errors.New("falkordb down")
	})
	rep := s.Run(context.Background(), []string{"graph"})
	if rep.OK {
		t.Errorf("expected aggregate fail")
	}
	if rep.Results[0].Error == "" || !strings.Contains(rep.Results[0].Error, "falkordb down") {
		t.Errorf("expected error surfaced; got %q", rep.Results[0].Error)
	}
}

func TestRun_PanicCaughtAndReported(t *testing.T) {
	s := New()
	_ = s.Register("oops", func(_ context.Context) (int, error) {
		panic("boom")
	})
	rep := s.Run(context.Background(), []string{"oops"})
	if rep.OK {
		t.Errorf("expected aggregate fail")
	}
	if !strings.Contains(rep.Results[0].Error, "panic") {
		t.Errorf("expected panic surfaced; got %q", rep.Results[0].Error)
	}
}

func TestRun_TouchedCountedPerTarget(t *testing.T) {
	s := New()
	_ = s.Register("a", func(_ context.Context) (int, error) { return 5, nil })
	_ = s.Register("b", func(_ context.Context) (int, error) { return 17, nil })
	rep := s.Run(context.Background(), []string{"a", "b"})
	if rep.Results[0].Touched != 5 || rep.Results[1].Touched != 17 {
		t.Errorf("touched mis-tracked: %+v", rep.Results)
	}
}

func TestRun_ContextCancellationPropagates(t *testing.T) {
	s := New()
	var observedCancel atomic.Bool
	_ = s.Register("slow", func(ctx context.Context) (int, error) {
		select {
		case <-time.After(time.Second):
			return 0, nil
		case <-ctx.Done():
			observedCancel.Store(true)
			return 0, ctx.Err()
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	rep := s.Run(ctx, []string{"slow"})
	if !observedCancel.Load() {
		t.Errorf("expected rebuilder to observe cancellation")
	}
	if rep.OK {
		t.Errorf("expected aggregate fail on cancel")
	}
}

func TestRun_EmptyServiceEmptyReport(t *testing.T) {
	s := New()
	rep := s.Run(context.Background(), nil)
	if !rep.OK {
		t.Errorf("empty service should not be a failure")
	}
	if len(rep.Results) != 0 {
		t.Errorf("expected no results; got %d", len(rep.Results))
	}
}
