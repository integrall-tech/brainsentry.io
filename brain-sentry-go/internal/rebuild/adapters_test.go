package rebuild

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
)

type fakeTenantLister struct {
	tenants []domain.Tenant
	err     error
}

func (f *fakeTenantLister) List(_ context.Context) ([]domain.Tenant, error) {
	return f.tenants, f.err
}

func TestCommunityAdapter_SumsAcrossTenants(t *testing.T) {
	tl := &fakeTenantLister{tenants: []domain.Tenant{
		{ID: "t1"}, {ID: "t2"}, {ID: "t3"},
	}}
	counts := map[string]int{"t1": 3, "t2": 5, "t3": 2}
	a := NewCommunityAdapter(tl, func(_ context.Context, tid string) (int, error) {
		return counts[tid], nil
	})
	n, err := a.DetectAllTenants(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if n != 10 {
		t.Errorf("expected sum 10; got %d", n)
	}
}

func TestCommunityAdapter_StopsOnFirstError(t *testing.T) {
	tl := &fakeTenantLister{tenants: []domain.Tenant{
		{ID: "t1"}, {ID: "t2"}, {ID: "t3"},
	}}
	calls := 0
	a := NewCommunityAdapter(tl, func(_ context.Context, tid string) (int, error) {
		calls++
		if tid == "t2" {
			return 0, errors.New("graph empty for tenant")
		}
		return 4, nil
	})
	_, err := a.DetectAllTenants(context.Background())
	if err == nil || !strings.Contains(err.Error(), "graph empty") {
		t.Errorf("expected error surfaced; got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected to stop after first failure (2 calls); got %d", calls)
	}
}

func TestCommunityAdapter_TenantListErrorPropagated(t *testing.T) {
	tl := &fakeTenantLister{err: errors.New("pg down")}
	a := NewCommunityAdapter(tl, func(_ context.Context, _ string) (int, error) { return 1, nil })
	_, err := a.DetectAllTenants(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pg down") {
		t.Errorf("expected list error propagated; got %v", err)
	}
}

func TestCommunityAdapter_NilSafetyReturnsZero(t *testing.T) {
	a := NewCommunityAdapter(nil, nil)
	n, err := a.DetectAllTenants(context.Background())
	if err != nil {
		t.Errorf("nil should not error; got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0; got %d", n)
	}
}

func TestCommunityAdapter_NoTenantsZero(t *testing.T) {
	a := NewCommunityAdapter(&fakeTenantLister{}, func(_ context.Context, _ string) (int, error) { return 99, nil })
	n, err := a.DetectAllTenants(context.Background())
	if err != nil {
		t.Errorf("empty list should not error; got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 when no tenants; got %d", n)
	}
}
