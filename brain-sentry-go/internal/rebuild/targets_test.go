package rebuild

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
)

// --- Fakes ---

type fakeLister struct {
	pages [][]domain.Memory
	err   error
}

func (f *fakeLister) List(_ context.Context, page, _ int) ([]domain.Memory, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	if page >= len(f.pages) {
		return nil, 0, nil
	}
	return f.pages[page], int64(len(f.pages[page])), nil
}

type fakeGraphSink struct {
	dropped     bool
	saved       []string
	relsTenants []string
	dropErr     error
	saveErr     error
	relsErr     error

	// Vector index bookkeeping. indexedAfterSaves records how many memories
	// were already written when the index was (re)created — the index must
	// come first, so this has to be 0.
	indexedDims       int
	indexCalls        int
	indexedAfterSaves int
	indexErr          error
}

func (f *fakeGraphSink) DropGraph(_ context.Context) error {
	f.dropped = true
	return f.dropErr
}

func (f *fakeGraphSink) EnsureVectorIndex(_ context.Context, dimensions int) error {
	f.indexCalls++
	f.indexedDims = dimensions
	f.indexedAfterSaves = len(f.saved)
	return f.indexErr
}

// plainGraphSink implements GraphSink WITHOUT VectorIndexEnsurer, to pin the
// optional-interface behaviour.
type plainGraphSink struct{ saved []string }

func (p *plainGraphSink) DropGraph(_ context.Context) error { return nil }
func (p *plainGraphSink) SaveToGraph(_ context.Context, m *domain.Memory) error {
	p.saved = append(p.saved, m.ID)
	return nil
}
func (p *plainGraphSink) CreateAllRelationships(_ context.Context, _ string) error { return nil }

func (f *fakeGraphSink) SaveToGraph(_ context.Context, m *domain.Memory) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, m.ID)
	return nil
}

func (f *fakeGraphSink) CreateAllRelationships(_ context.Context, tenantID string) error {
	if f.relsErr != nil {
		return f.relsErr
	}
	f.relsTenants = append(f.relsTenants, tenantID)
	return nil
}

type fakeNuller struct {
	count int64
	err   error
}

func (f *fakeNuller) NullifyAllEmbeddings(_ context.Context) (int64, error) {
	return f.count, f.err
}

type fakeDetector struct {
	count int
	err   error
}

func (f *fakeDetector) DetectAllTenants(_ context.Context) (int, error) {
	return f.count, f.err
}

type fakeWiper struct {
	count int64
	err   error
}

func (f *fakeWiper) WipeAllContextSummaries(_ context.Context) (int64, error) {
	return f.count, f.err
}

// --- GraphRebuilder ---

func TestGraphRebuilder_DropsThenInsertsAllPagesThenEdgesPerTenant(t *testing.T) {
	lister := &fakeLister{
		pages: [][]domain.Memory{
			{
				{ID: "m1", TenantID: "t1"},
				{ID: "m2", TenantID: "t1"},
			},
			{
				{ID: "m3", TenantID: "t2"},
			},
		},
	}
	sink := &fakeGraphSink{}
	n, err := GraphRebuilder(lister, sink, 0)(context.Background())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !sink.dropped {
		t.Errorf("expected drop")
	}
	if n != 3 {
		t.Errorf("expected touched=3; got %d", n)
	}
	if strings.Join(sink.saved, ",") != "m1,m2,m3" {
		t.Errorf("expected all memories saved in page order; got %v", sink.saved)
	}
	tenants := map[string]bool{}
	for _, t := range sink.relsTenants {
		tenants[t] = true
	}
	if !tenants["t1"] || !tenants["t2"] {
		t.Errorf("expected edges rebuilt for both tenants; got %v", sink.relsTenants)
	}
}

func TestGraphRebuilder_NilArgsErrors(t *testing.T) {
	if _, err := GraphRebuilder(nil, nil, 0)(context.Background()); err == nil {
		t.Errorf("expected error for nil deps")
	}
}

func TestGraphRebuilder_DropFailureAborts(t *testing.T) {
	sink := &fakeGraphSink{dropErr: errors.New("boom")}
	_, err := GraphRebuilder(&fakeLister{}, sink, 0)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "drop graph") {
		t.Errorf("expected drop error; got %v", err)
	}
}

func TestGraphRebuilder_SaveFailureAbortsWithCount(t *testing.T) {
	sink := &fakeGraphSink{saveErr: errors.New("falkor down")}
	lister := &fakeLister{pages: [][]domain.Memory{{{ID: "m1", TenantID: "t1"}}}}
	n, err := GraphRebuilder(lister, sink, 0)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "save memory") {
		t.Errorf("expected save error; got %v", err)
	}
	if n != 0 {
		t.Errorf("touched should be 0 when first save fails; got %d", n)
	}
}

func TestGraphRebuilder_StopsAtPartialPage(t *testing.T) {
	// A page smaller than pageSize signals "no more pages."
	short := make([]domain.Memory, 3)
	for i := range short {
		short[i] = domain.Memory{ID: "m" + string(rune('0'+i)), TenantID: "t1"}
	}
	lister := &fakeLister{pages: [][]domain.Memory{short}}
	sink := &fakeGraphSink{}
	n, err := GraphRebuilder(lister, sink, 0)(context.Background())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3; got %d", n)
	}
}

// --- EmbeddingsRebuilder ---

func TestEmbeddingsRebuilder_PassesThroughCount(t *testing.T) {
	n, err := EmbeddingsRebuilder(&fakeNuller{count: 42})(context.Background())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 42 {
		t.Errorf("expected 42; got %d", n)
	}
}

func TestEmbeddingsRebuilder_ErrorPropagated(t *testing.T) {
	_, err := EmbeddingsRebuilder(&fakeNuller{err: errors.New("pg lock timeout")})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "nullify embeddings") {
		t.Errorf("expected nullify error; got %v", err)
	}
}

func TestEmbeddingsRebuilder_NilNuller(t *testing.T) {
	if _, err := EmbeddingsRebuilder(nil)(context.Background()); err == nil {
		t.Errorf("expected error for nil nuller")
	}
}

// --- CommunitiesRebuilder ---

func TestCommunitiesRebuilder_PassesThroughCount(t *testing.T) {
	n, err := CommunitiesRebuilder(&fakeDetector{count: 7})(context.Background())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7; got %d", n)
	}
}

func TestCommunitiesRebuilder_ErrorPropagated(t *testing.T) {
	_, err := CommunitiesRebuilder(&fakeDetector{err: errors.New("graph empty")})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "detect communities") {
		t.Errorf("expected error wrapped; got %v", err)
	}
}

// --- CompressRebuilder ---

func TestCompressRebuilder_PassesThroughCount(t *testing.T) {
	n, err := CompressRebuilder(&fakeWiper{count: 19})(context.Background())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 19 {
		t.Errorf("expected 19; got %d", n)
	}
}

func TestCompressRebuilder_ErrorPropagated(t *testing.T) {
	_, err := CompressRebuilder(&fakeWiper{err: errors.New("pg lock")})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "wipe context summaries") {
		t.Errorf("expected wipe error; got %v", err)
	}
}

// The bug: DropGraph removes the vector index along with the graph, and the
// index was only ever created at server boot — so every rebuild left vector
// search silently degraded until the next restart.
func TestGraphRebuilder_RecreatesVectorIndexBeforeInserting(t *testing.T) {
	lister := &fakeLister{pages: [][]domain.Memory{{{ID: "m1", TenantID: "t1"}}}}
	sink := &fakeGraphSink{}

	if _, err := GraphRebuilder(lister, sink, 1536)(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if sink.indexCalls != 1 {
		t.Errorf("expected the index to be recreated once, got %d calls", sink.indexCalls)
	}
	if sink.indexedDims != 1536 {
		t.Errorf("index created with %d dimensions, want 1536", sink.indexedDims)
	}
	// Ordering matters: an index built after the nodes would not cover them.
	if sink.indexedAfterSaves != 0 {
		t.Errorf("index was created after %d saves; it must come first", sink.indexedAfterSaves)
	}
}

func TestGraphRebuilder_SkipsIndexWhenDimensionsUnset(t *testing.T) {
	sink := &fakeGraphSink{}
	if _, err := GraphRebuilder(&fakeLister{}, sink, 0)(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if sink.indexCalls != 0 {
		t.Errorf("dimensions=0 must skip the index, got %d calls", sink.indexCalls)
	}
}

// A sink that cannot manage indexes must still rebuild normally.
func TestGraphRebuilder_SinkWithoutIndexSupportStillRebuilds(t *testing.T) {
	lister := &fakeLister{pages: [][]domain.Memory{{{ID: "m1", TenantID: "t1"}}}}
	sink := &plainGraphSink{}

	n, err := GraphRebuilder(lister, sink, 1536)(context.Background())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 1 || len(sink.saved) != 1 {
		t.Errorf("expected 1 memory rebuilt, got n=%d saved=%v", n, sink.saved)
	}
}

// A failed index recreation must abort: continuing would repopulate the graph
// with no index, which is exactly the silent-degradation state we're fixing.
func TestGraphRebuilder_IndexFailureAborts(t *testing.T) {
	sink := &fakeGraphSink{indexErr: errors.New("no index for you")}
	_, err := GraphRebuilder(&fakeLister{}, sink, 1536)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "recreate vector index") {
		t.Errorf("expected index error; got %v", err)
	}
}
