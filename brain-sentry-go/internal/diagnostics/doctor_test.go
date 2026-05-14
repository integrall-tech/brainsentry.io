package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// --- aggregate / status math ---

func TestAggregate_AllOK(t *testing.T) {
	results := []CheckResult{
		{Name: "a", Status: StatusOK},
		{Name: "b", Status: StatusOK},
	}
	st, sum := aggregate(results)
	if st != StatusOK {
		t.Errorf("expected StatusOK; got %s", st)
	}
	if sum.OK != 2 {
		t.Errorf("expected 2 ok; got %d", sum.OK)
	}
}

func TestAggregate_FailDominatesWarn(t *testing.T) {
	results := []CheckResult{
		{Name: "a", Status: StatusWarn},
		{Name: "b", Status: StatusFail},
		{Name: "c", Status: StatusOK},
	}
	st, sum := aggregate(results)
	if st != StatusFail {
		t.Errorf("expected fail to dominate warn; got %s", st)
	}
	if sum.Fail != 1 || sum.Warn != 1 || sum.OK != 1 {
		t.Errorf("counters off: %+v", sum)
	}
}

func TestAggregate_SkipDoesNotAffectStatus(t *testing.T) {
	results := []CheckResult{
		{Name: "a", Status: StatusOK},
		{Name: "b", Status: StatusSkip},
	}
	st, sum := aggregate(results)
	if st != StatusOK {
		t.Errorf("expected ok; got %s", st)
	}
	if sum.Skip != 1 {
		t.Errorf("expected skip=1; got %+v", sum)
	}
}

// --- Doctor.Run orchestration ---

type fakeChecker struct {
	name string
	res  CheckResult
	wait time.Duration
}

func (f *fakeChecker) Name() string { return f.name }
func (f *fakeChecker) Check(ctx context.Context) CheckResult {
	if f.wait > 0 {
		select {
		case <-time.After(f.wait):
		case <-ctx.Done():
			return CheckResult{Name: f.name, Status: StatusFail, Message: "timed out"}
		}
	}
	r := f.res
	r.Name = f.name
	return r
}

func TestDoctor_RunReturnsAllChecksSorted(t *testing.T) {
	d := New([]Checker{
		&fakeChecker{name: "z-last", res: CheckResult{Status: StatusOK, Message: "fine"}},
		&fakeChecker{name: "a-first", res: CheckResult{Status: StatusOK, Message: "fine"}},
		&fakeChecker{name: "m-mid", res: CheckResult{Status: StatusWarn, Message: "soft issue"}},
	}, 2*time.Second)
	rep := d.Run(context.Background())
	if len(rep.Checks) != 3 {
		t.Fatalf("expected 3 checks; got %d", len(rep.Checks))
	}
	if rep.Checks[0].Name != "a-first" || rep.Checks[2].Name != "z-last" {
		t.Errorf("expected stable alphabetical order; got %v", names(rep.Checks))
	}
	if rep.Status != StatusWarn {
		t.Errorf("expected aggregate warn; got %s", rep.Status)
	}
}

func TestDoctor_PerCheckTimeoutHonored(t *testing.T) {
	d := New([]Checker{
		&fakeChecker{name: "slow", wait: 200 * time.Millisecond},
	}, 50*time.Millisecond)
	rep := d.Run(context.Background())
	if rep.Status != StatusFail {
		t.Errorf("expected fail from timeout; got %s, checks=%+v", rep.Status, rep.Checks)
	}
}

func TestDoctor_ExitCode(t *testing.T) {
	d := New([]Checker{
		&fakeChecker{name: "a", res: CheckResult{Status: StatusOK}},
	}, time.Second)
	if d.Run(context.Background()).ExitCode() != 0 {
		t.Errorf("clean run should be exit 0")
	}
	d2 := New([]Checker{
		&fakeChecker{name: "a", res: CheckResult{Status: StatusFail, Message: "boom"}},
	}, time.Second)
	if d2.Run(context.Background()).ExitCode() != 1 {
		t.Errorf("failing run should be exit 1")
	}
}

func TestReport_FormatTextHumanReadable(t *testing.T) {
	d := New([]Checker{
		&fakeChecker{name: "redis", res: CheckResult{Status: StatusOK, Message: "reachable"}},
		&fakeChecker{name: "openrouter", res: CheckResult{Status: StatusFail, Message: "401", Hint: "check key"}},
	}, time.Second)
	out := d.Run(context.Background()).FormatText()
	for _, want := range []string{"brainsentry doctor", "[ok] redis", "[fail] openrouter", "hint:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output; got %q", want, out)
		}
	}
}

func TestReport_JSONStable(t *testing.T) {
	d := New([]Checker{
		&fakeChecker{name: "x", res: CheckResult{Status: StatusOK, Message: "fine"}},
	}, time.Second)
	rep := d.Run(context.Background())
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"status":"ok"`) {
		t.Errorf("expected status field in json; got %s", b)
	}
}

// --- TCPChecker ---

func TestTCPChecker_Reachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	c := &TCPChecker{CheckName: "loopback", Host: host, Port: port}
	r := c.Check(context.Background())
	if r.Status != StatusOK {
		t.Errorf("expected ok for live listener; got %s (%s)", r.Status, r.Detail)
	}
}

func TestTCPChecker_Unreachable(t *testing.T) {
	c := &TCPChecker{CheckName: "ghost", Host: "127.0.0.1", Port: 1, Hint: "start the service"}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	r := c.Check(ctx)
	if r.Status != StatusFail {
		t.Errorf("expected fail; got %s", r.Status)
	}
	if r.Hint == "" {
		t.Errorf("expected hint to be propagated")
	}
}

// --- HTTPChecker ---

func TestHTTPChecker_2xxIsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := &HTTPChecker{CheckName: "test", URL: srv.URL, Method: "GET"}
	r := c.Check(context.Background())
	if r.Status != StatusOK {
		t.Errorf("expected ok; got %s (%s)", r.Status, r.Detail)
	}
}

func TestHTTPChecker_401IsWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := &HTTPChecker{CheckName: "auth", URL: srv.URL, Method: "GET"}
	r := c.Check(context.Background())
	if r.Status != StatusWarn {
		t.Errorf("expected warn for 401; got %s", r.Status)
	}
	if !strings.Contains(strings.ToLower(r.Hint), "key") {
		t.Errorf("expected api-key hint; got %q", r.Hint)
	}
}

func TestHTTPChecker_500IsFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := &HTTPChecker{CheckName: "boom", URL: srv.URL, Method: "GET"}
	r := c.Check(context.Background())
	if r.Status != StatusFail {
		t.Errorf("expected fail; got %s", r.Status)
	}
}

func TestHTTPChecker_EmptyURLSkips(t *testing.T) {
	c := &HTTPChecker{CheckName: "nope"}
	r := c.Check(context.Background())
	if r.Status != StatusSkip {
		t.Errorf("expected skip for empty URL; got %s", r.Status)
	}
}

// --- SchemaVersionChecker ---

func TestSchemaVersionChecker_AtVersionPasses(t *testing.T) {
	c := &SchemaVersionChecker{
		CheckName: "schema", MinVersion: "0010",
		Query: func(ctx context.Context) (string, error) { return "0023", nil },
	}
	r := c.Check(context.Background())
	if r.Status != StatusOK {
		t.Errorf("expected ok at higher schema; got %s (%s)", r.Status, r.Detail)
	}
}

func TestSchemaVersionChecker_BehindVersionFails(t *testing.T) {
	c := &SchemaVersionChecker{
		CheckName: "schema", MinVersion: "0023",
		Query: func(ctx context.Context) (string, error) { return "0010", nil },
	}
	r := c.Check(context.Background())
	if r.Status != StatusFail {
		t.Errorf("expected fail for outdated schema; got %s", r.Status)
	}
	if r.Hint == "" {
		t.Errorf("expected migrate hint")
	}
}

func TestSchemaVersionChecker_QueryErrorFails(t *testing.T) {
	c := &SchemaVersionChecker{
		CheckName: "schema", MinVersion: "0001",
		Query: func(ctx context.Context) (string, error) { return "", errors.New("conn refused") },
	}
	r := c.Check(context.Background())
	if r.Status != StatusFail {
		t.Errorf("expected fail when query errors; got %s", r.Status)
	}
}

// --- helpers ---

func names(results []CheckResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Name
	}
	return out
}
