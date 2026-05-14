package rebuild

import (
	"context"

	"github.com/integraltech/brainsentry/internal/domain"
)

// --- Adapters: thin shims connecting the rebuild interfaces to the
//     concrete repository / service surfaces in cmd/server/main.go.
//
// These live in this package (rather than at the call site) so the wiring
// stays one-import: cmd/server only needs to call rebuild.NewCommunityAdapter
// instead of building the closure inline. Tests use the underlying interface
// directly (they don't need the adapter).

// TenantLister is the smallest dependency the community adapter needs. The
// concrete TenantRepository.List implements it directly.
type TenantLister interface {
	List(ctx context.Context) ([]domain.Tenant, error)
}

// CommunityCounter is what cmd/server feeds in: a closure that runs
// community detection for one tenant and returns "communities found".
// We keep this as a closure (not a typed service) so the rebuild package
// has zero upward dependency on internal/service.
type CommunityCounter func(ctx context.Context, tenantID string) (int, error)

// communityAdapter walks every tenant and sums per-tenant community
// counts. The rebuild report carries the sum; operators care about
// graph-level scale, not per-tenant breakdown (that lives on the
// communities admin page).
type communityAdapter struct {
	tenants TenantLister
	count   CommunityCounter
}

// NewCommunityAdapter builds the adapter the rebuild service needs.
func NewCommunityAdapter(tenants TenantLister, count CommunityCounter) CommunityRebuilder {
	return &communityAdapter{tenants: tenants, count: count}
}

func (c *communityAdapter) DetectAllTenants(ctx context.Context) (int, error) {
	if c.tenants == nil || c.count == nil {
		return 0, nil
	}
	tenants, err := c.tenants.List(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, t := range tenants {
		n, err := c.count(ctx, t.ID)
		if err != nil {
			// surface the first failure so the report has a real cause —
			// stopping after the first prevents misleading partial counts.
			return total, err
		}
		total += n
	}
	return total, nil
}
