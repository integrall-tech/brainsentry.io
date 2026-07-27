package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

type fakePurgeRepo struct {
	resolved    []string
	resolveErr  error
	lastScope   postgres.PurgeScope
	purgeCalls  []bool // dryRun flag per call
	purgeCounts postgres.PurgeCounts
	redactCalls []bool
	expiredIDs  []string
	lastCutoff  time.Time
	lastLimit   int
	// order records the sequence of operations, because their ORDER is the
	// property under test: redaction resolves audit rows through link tables
	// that the purge deletes.
	order []string
}

func (f *fakePurgeRepo) ResolveScope(_ context.Context, scope postgres.PurgeScope) ([]string, error) {
	f.lastScope = scope
	return f.resolved, f.resolveErr
}

func (f *fakePurgeRepo) PurgeMemories(_ context.Context, ids []string, dryRun bool) (postgres.PurgeCounts, error) {
	f.order = append(f.order, "purge")
	f.purgeCalls = append(f.purgeCalls, dryRun)
	if f.purgeCounts != nil {
		return f.purgeCounts, nil
	}
	return postgres.PurgeCounts{"memories": int64(len(ids))}, nil
}

func (f *fakePurgeRepo) RedactAuditContent(_ context.Context, ids []string, dryRun bool) (int64, error) {
	f.order = append(f.order, "redact")
	f.redactCalls = append(f.redactCalls, dryRun)
	return int64(len(ids)), nil
}

func (f *fakePurgeRepo) FindExpiredBefore(_ context.Context, cutoff time.Time, limit int) ([]string, error) {
	f.lastCutoff = cutoff
	f.lastLimit = limit
	return f.expiredIDs, nil
}

type fakeGraphPurger struct {
	calls int
	err   error
}

func (f *fakeGraphPurger) PurgeMemoryNodes(_ context.Context, _ string, _ []string) error {
	f.calls++
	return f.err
}

type fakeTenantReader struct{ settings string }

func (f *fakeTenantReader) FindByID(_ context.Context, id string) (*domain.Tenant, error) {
	return &domain.Tenant{ID: id, Settings: json.RawMessage(f.settings)}, nil
}

type fakeReceipts struct{ written []*domain.ErasureReceipt }

func (f *fakeReceipts) Create(_ context.Context, r *domain.ErasureReceipt) error {
	f.written = append(f.written, r)
	return nil
}

func newRetention(repo *fakePurgeRepo, graph *fakeGraphPurger, settings string) (*RetentionService, *fakeReceipts) {
	receipts := &fakeReceipts{}
	tenants := &fakeTenantReader{settings: settings}
	return NewRetentionService(repo, graph, tenants, receipts), receipts
}

// The single most dangerous input: an empty scope would match every memory in
// the tenant.
func TestEraseSubject_RefusesEmptyScope(t *testing.T) {
	repo := &fakePurgeRepo{}
	svc, _ := newRetention(repo, nil, "{}")

	if _, err := svc.EraseSubject(context.Background(), postgres.PurgeScope{}, "pedido", "u", true); err == nil {
		t.Fatal("an empty scope must be refused")
	}
	if len(repo.purgeCalls) != 0 {
		t.Error("nothing must be purged when the scope is refused")
	}
}

func TestEraseSubject_RequiresReason(t *testing.T) {
	svc, _ := newRetention(&fakePurgeRepo{}, nil, "{}")
	_, err := svc.EraseSubject(context.Background(), postgres.PurgeScope{Tag: "cliente:acme"}, "", "u", true)
	if err == nil {
		t.Error("a receipt with no reason cannot answer why the removal happened; must be refused")
	}
}

// Dry run is the default, and it must reach the repository with dryRun=true —
// not skip it, or the reported counts would be a guess.
func TestEraseSubject_DryRunByDefault(t *testing.T) {
	repo := &fakePurgeRepo{resolved: []string{"m1", "m2"}}
	graph := &fakeGraphPurger{}
	svc, receipts := newRetention(repo, graph, "{}")

	result, err := svc.EraseSubject(context.Background(),
		postgres.PurgeScope{Tag: "cliente:acme"}, "pedido do titular", "u", false)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}

	if result.Executed {
		t.Error("without confirm the run must not be marked executed")
	}
	if len(repo.purgeCalls) != 1 || !repo.purgeCalls[0] {
		t.Errorf("expected exactly one purge with dryRun=true, got %v", repo.purgeCalls)
	}
	if graph.calls != 0 {
		t.Error("a dry run must not touch the graph")
	}
	if len(receipts.written) != 1 || receipts.written[0].Executed {
		t.Error("the plan must be receipted too, marked as not executed")
	}
}

func TestEraseSubject_ConfirmExecutesEverySurface(t *testing.T) {
	repo := &fakePurgeRepo{resolved: []string{"m1"}}
	graph := &fakeGraphPurger{}
	svc, receipts := newRetention(repo, graph, "{}")

	result, err := svc.EraseSubject(context.Background(),
		postgres.PurgeScope{Tag: "cliente:acme"}, "pedido do titular", "service:key-1", true)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}

	if !result.Executed {
		t.Error("confirm must mark the run executed")
	}
	if len(repo.purgeCalls) != 1 || repo.purgeCalls[0] {
		t.Errorf("expected a destructive purge (dryRun=false), got %v", repo.purgeCalls)
	}
	if len(repo.redactCalls) != 1 || repo.redactCalls[0] {
		t.Error("audit content must be redacted for real on a confirmed run")
	}
	if graph.calls != 1 {
		t.Error("the graph carries content too and must be purged")
	}
	if !result.GraphPurged {
		t.Error("graphPurged should be true when the graph purge succeeded")
	}
	if len(receipts.written) != 1 || receipts.written[0].RequestedBy != "service:key-1" {
		t.Errorf("receipt must record who asked: %+v", receipts.written)
	}
}

// A FalkorDB outage must not report "erasure failed" for something that
// mostly succeeded — but it must say what is left.
func TestEraseSubject_GraphFailureIsReportedNotFatal(t *testing.T) {
	repo := &fakePurgeRepo{resolved: []string{"m1"}}
	graph := &fakeGraphPurger{err: errors.New("falkordb down")}
	svc, _ := newRetention(repo, graph, "{}")

	result, err := svc.EraseSubject(context.Background(),
		postgres.PurgeScope{Tag: "cliente:acme"}, "pedido", "u", true)
	if err != nil {
		t.Fatalf("a graph failure must not fail the whole erasure: %v", err)
	}
	if result.GraphPurged {
		t.Error("graphPurged must be false when the graph purge failed")
	}
	if result.GraphError == "" {
		t.Error("the caller must be told the graph still holds data")
	}
}

// "We looked and found nothing" is an answer a data subject may need on paper.
func TestEraseSubject_NoMatchStillReceipts(t *testing.T) {
	repo := &fakePurgeRepo{resolved: nil}
	svc, receipts := newRetention(repo, &fakeGraphPurger{}, "{}")

	result, err := svc.EraseSubject(context.Background(),
		postgres.PurgeScope{Tag: "cliente:desconhecido"}, "pedido", "u", true)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if result.Matched != 0 {
		t.Errorf("expected 0 matches, got %d", result.Matched)
	}
	if len(receipts.written) != 1 {
		t.Error("an empty result must still be receipted")
	}
	if len(repo.purgeCalls) != 0 {
		t.Error("nothing to purge means no purge call")
	}
}

// --- Retention policy ---

// A tenant that never declared a policy must keep everything. Deleting on a
// default nobody chose is the unrecoverable direction to be wrong in.
func TestRunRetention_NoPolicyIsNoOp(t *testing.T) {
	repo := &fakePurgeRepo{expiredIDs: []string{"m1", "m2"}}
	svc, _ := newRetention(repo, &fakeGraphPurger{}, `{}`)

	result, err := svc.RunRetention(context.Background(), "u", true)
	if err != nil {
		t.Fatalf("retention: %v", err)
	}
	if result.Executed {
		t.Error("a tenant without a policy must not execute anything")
	}
	if len(repo.purgeCalls) != 0 {
		t.Error("no policy must mean no purge")
	}
}

func TestRunRetention_UsesTenantPolicy(t *testing.T) {
	repo := &fakePurgeRepo{expiredIDs: []string{"m1"}}
	svc, receipts := newRetention(repo, &fakeGraphPurger{},
		`{"retention":{"purgeAfterValidToDays":90,"maxPerRun":25}}`)

	before := time.Now().AddDate(0, 0, -90)
	if _, err := svc.RunRetention(context.Background(), "u", true); err != nil {
		t.Fatalf("retention: %v", err)
	}

	if repo.lastLimit != 25 {
		t.Errorf("maxPerRun not honoured: got %d", repo.lastLimit)
	}
	// The cutoff must be ~90 days back; allow a second of drift for the clock
	// between the test and the service.
	if repo.lastCutoff.Sub(before) > time.Second || before.Sub(repo.lastCutoff) > time.Second {
		t.Errorf("cutoff = %v, expected ~%v", repo.lastCutoff, before)
	}
	if len(receipts.written) != 1 || receipts.written[0].Kind != "retention" {
		t.Errorf("expected a retention receipt: %+v", receipts.written)
	}
}

func TestRunRetention_DefaultsMaxPerRun(t *testing.T) {
	repo := &fakePurgeRepo{expiredIDs: []string{"m1"}}
	svc, _ := newRetention(repo, &fakeGraphPurger{}, `{"retention":{"purgeAfterValidToDays":30}}`)

	if _, err := svc.RunRetention(context.Background(), "u", false); err != nil {
		t.Fatalf("retention: %v", err)
	}
	if repo.lastLimit != defaultMaxPerRun {
		t.Errorf("expected the default bound %d, got %d", defaultMaxPerRun, repo.lastLimit)
	}
}

// --- Policy parsing ---

func TestRetentionPolicyFromSettings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings string
		want     int
	}{
		{"absent", `{}`, 0},
		{"empty", ``, 0},
		{"malformed", `{not json`, 0},
		{"other keys only", `{"foo":"bar"}`, 0},
		{"declared", `{"retention":{"purgeAfterValidToDays":45}}`, 45},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.RetentionPolicyFromSettings([]byte(tc.settings))
			if got.PurgeAfterValidToDays != tc.want {
				t.Errorf("PurgeAfterValidToDays = %d, want %d", got.PurgeAfterValidToDays, tc.want)
			}
			if got.Enabled() != (tc.want > 0) {
				t.Errorf("Enabled() = %v for %q", got.Enabled(), tc.settings)
			}
		})
	}
}

// Regression: the purge deletes the audit_memories_* link rows, and the
// redaction finds its audit_logs rows THROUGH those links. Purging first
// silently leaves the subject's text in audit_logs.user_request — verified
// against a real schema, where the audit row still read the original text
// after a "complete" erasure.
func TestEraseSubject_RedactsAuditBeforePurging(t *testing.T) {
	repo := &fakePurgeRepo{resolved: []string{"m1"}}
	svc, _ := newRetention(repo, &fakeGraphPurger{}, "{}")

	if _, err := svc.EraseSubject(context.Background(),
		postgres.PurgeScope{Tag: "cliente:acme"}, "pedido", "u", true); err != nil {
		t.Fatalf("erase: %v", err)
	}

	if len(repo.order) != 2 {
		t.Fatalf("expected redact + purge, got %v", repo.order)
	}
	if repo.order[0] != "redact" {
		t.Errorf("audit must be redacted BEFORE the purge removes the links it joins through; order was %v", repo.order)
	}
}
