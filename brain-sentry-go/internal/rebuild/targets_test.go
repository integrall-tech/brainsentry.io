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
}

func (f *fakeGraphSink) DropGraph(_ context.Context) error {
	f.dropped = true
	return f.dropErr
}

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
	n, err := GraphRebuilder(lister, sink)(context.Background())
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
	if _, err := GraphRebuilder(nil, nil)(context.Background()); err == nil {
		t.Errorf("expected error for nil deps")
	}
}

func TestGraphRebuilder_DropFailureAborts(t *testing.T) {
	sink := &fakeGraphSink{dropErr: errors.New("boom")}
	_, err := GraphRebuilder(&fakeLister{}, sink)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "drop graph") {
		t.Errorf("expected drop error; got %v", err)
	}
}

func TestGraphRebuilder_SaveFailureAbortsWithCount(t *testing.T) {
	sink := &fakeGraphSink{saveErr: errors.New("falkor down")}
	lister := &fakeLister{pages: [][]domain.Memory{{{ID: "m1", TenantID: "t1"}}}}
	n, err := GraphRebuilder(lister, sink)(context.Background())
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
	n, err := GraphRebuilder(lister, sink)(context.Background())
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
