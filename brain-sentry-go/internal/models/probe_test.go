package models

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProber struct {
	calls   int32
	respond func(model string) error
}

func (f *fakeProber) Probe(_ context.Context, model string) error {
	atomic.AddInt32(&f.calls, 1)
	if f.respond == nil {
		return nil
	}
	return f.respond(model)
}

func TestRunDoctor_AllPass(t *testing.T) {
	cfg := Config{Default: "default-m"}
	rep := RunDoctor(context.Background(), cfg, &fakeProber{}, time.Second)
	if !rep.OK {
		t.Errorf("expected ok aggregate; got %+v", rep)
	}
	if len(rep.Results) != len(AllTiers()) {
		t.Errorf("expected one result per tier; got %d", len(rep.Results))
	}
}

func TestRunDoctor_ClassifiesNotFound(t *testing.T) {
	rep := RunDoctor(context.Background(), Config{Default: "phantom"}, &fakeProber{
		respond: func(_ string) error { return errors.New("HTTP 404 model not found") },
	}, time.Second)
	if rep.OK {
		t.Errorf("aggregate should be false")
	}
	for _, r := range rep.Results {
		if r.Failure != FailureModelNotFound {
			t.Errorf("expected model_not_found; got %s for tier %s", r.Failure, r.Tier)
		}
	}
}

func TestRunDoctor_ClassifiesAuth(t *testing.T) {
	rep := RunDoctor(context.Background(), Config{Default: "x"}, &fakeProber{
		respond: func(_ string) error { return errors.New("HTTP 401 unauthorized") },
	}, time.Second)
	for _, r := range rep.Results {
		if r.Failure != FailureAuth {
			t.Errorf("expected auth; got %s", r.Failure)
		}
	}
}

func TestRunDoctor_ClassifiesRateLimit(t *testing.T) {
	rep := RunDoctor(context.Background(), Config{Default: "x"}, &fakeProber{
		respond: func(_ string) error { return errors.New("HTTP 429 rate limit exceeded") },
	}, time.Second)
	for _, r := range rep.Results {
		if r.Failure != FailureRateLimit {
			t.Errorf("expected rate_limit; got %s", r.Failure)
		}
	}
}

func TestRunDoctor_ClassifiesNetwork(t *testing.T) {
	rep := RunDoctor(context.Background(), Config{Default: "x"}, &fakeProber{
		respond: func(_ string) error { return errors.New("dial tcp 127.0.0.1:443: connection refused") },
	}, time.Second)
	for _, r := range rep.Results {
		if r.Failure != FailureNetwork {
			t.Errorf("expected network; got %s (detail=%s)", r.Failure, r.Detail)
		}
	}
}

func TestRunDoctor_ClassifiesTimeout(t *testing.T) {
	cfg := Config{Default: "x"}
	rep := RunDoctor(context.Background(), cfg, &fakeProber{
		respond: func(_ string) error { return context.DeadlineExceeded },
	}, time.Second)
	for _, r := range rep.Results {
		if r.Failure != FailureTimeout {
			t.Errorf("expected timeout; got %s", r.Failure)
		}
	}
}

func TestRunDoctor_FailsWhenTierUnresolvable(t *testing.T) {
	saved := TierDefaults
	t.Cleanup(func() { TierDefaults = saved })
	TierDefaults = map[Tier]string{}
	rep := RunDoctor(context.Background(), Config{}, &fakeProber{}, time.Second)
	if rep.OK {
		t.Errorf("expected fail when no tier resolves")
	}
	for _, r := range rep.Results {
		if r.Failure != FailureInvalidRequest {
			t.Errorf("expected invalid_request for unresolvable tier; got %s", r.Failure)
		}
	}
}

func TestHTTPProber_2xxIsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer srv.Close()
	p := &HTTPProber{BaseURL: srv.URL, APIKey: "key"}
	if err := p.Probe(context.Background(), "any-model"); err != nil {
		t.Errorf("expected ok; got %v", err)
	}
}

func TestHTTPProber_404IsModelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	p := &HTTPProber{BaseURL: srv.URL, APIKey: "key"}
	err := p.Probe(context.Background(), "phantom")
	if err == nil {
		t.Fatalf("expected error")
	}
	kind, _ := classify(err)
	if kind != FailureModelNotFound {
		t.Errorf("expected model_not_found; got %s", kind)
	}
}

func TestHTTPProber_PostsModelInBody(t *testing.T) {
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 200)
		n, _ := r.Body.Read(buf)
		seenBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	p := &HTTPProber{BaseURL: srv.URL, APIKey: "key"}
	_ = p.Probe(context.Background(), "claude-3-5-sonnet")
	if !strings.Contains(seenBody, `"model":"claude-3-5-sonnet"`) {
		t.Errorf("expected model field in body; got %q", seenBody)
	}
	if !strings.Contains(seenBody, `"max_tokens":1`) {
		t.Errorf("expected max_tokens=1 in body; got %q", seenBody)
	}
}

func TestJsonString_EscapesQuotesAndBackslash(t *testing.T) {
	if jsonString(`he said "hi"`) != `"he said \"hi\""` {
		t.Errorf("quote escape broken: got %s", jsonString(`he said "hi"`))
	}
	if jsonString(`a\b`) != `"a\\b"` {
		t.Errorf("backslash escape broken")
	}
}
