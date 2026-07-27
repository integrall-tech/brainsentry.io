package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

type fakeAPIKeyRepo struct {
	byPrefix   map[string][]*domain.APIKey
	created    []*domain.APIKey
	touched    []string
	lookupErr  error
	revokedIDs []string
}

func newFakeAPIKeyRepo() *fakeAPIKeyRepo {
	return &fakeAPIKeyRepo{byPrefix: map[string][]*domain.APIKey{}}
}

func (f *fakeAPIKeyRepo) Create(_ context.Context, k *domain.APIKey) error {
	if k.ID == "" {
		k.ID = "generated-id"
	}
	f.created = append(f.created, k)
	f.byPrefix[k.KeyPrefix] = append(f.byPrefix[k.KeyPrefix], k)
	return nil
}

func (f *fakeAPIKeyRepo) FindCandidatesByPrefix(_ context.Context, prefix string) ([]*domain.APIKey, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return f.byPrefix[prefix], nil
}

func (f *fakeAPIKeyRepo) ListByTenant(_ context.Context, tenantID string) ([]*domain.APIKey, error) {
	var out []*domain.APIKey
	for _, keys := range f.byPrefix {
		for _, k := range keys {
			if k.TenantID == tenantID {
				out = append(out, k)
			}
		}
	}
	return out, nil
}

func (f *fakeAPIKeyRepo) FindByID(_ context.Context, id string) (*domain.APIKey, error) {
	for _, keys := range f.byPrefix {
		for _, k := range keys {
			if k.ID == id {
				return k, nil
			}
		}
	}
	return nil, errors.New("api key not found")
}

func (f *fakeAPIKeyRepo) Revoke(_ context.Context, id string) error {
	f.revokedIDs = append(f.revokedIDs, id)
	return nil
}

func (f *fakeAPIKeyRepo) TouchLastUsed(_ context.Context, id string) error {
	f.touched = append(f.touched, id)
	return nil
}

func TestAPIKeyService_CreateStoresHashNotSecret(t *testing.T) {
	repo := newFakeAPIKeyRepo()
	svc := NewAPIKeyService(repo)

	created, err := svc.Create(context.Background(), "tenant-a", "core", "user-1", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Secret == "" {
		t.Fatal("the plaintext secret must be returned once")
	}
	stored := repo.created[0]
	if stored.KeyHash == created.Secret {
		t.Fatal("the secret must never be stored in clear")
	}
	if stored.KeyHash != domain.HashAPIKeySecret(created.Secret) {
		t.Error("stored hash does not correspond to the returned secret")
	}
	if stored.TenantID != "tenant-a" {
		t.Errorf("key bound to %q, want tenant-a", stored.TenantID)
	}
}

func TestAPIKeyService_CreateRequiresTenantAndName(t *testing.T) {
	svc := NewAPIKeyService(newFakeAPIKeyRepo())
	if _, err := svc.Create(context.Background(), "", "core", "u", nil); err == nil {
		t.Error("an empty tenant must be refused — a credential must never be issued unscoped")
	}
	if _, err := svc.Create(context.Background(), "tenant-a", "  ", "u", nil); err == nil {
		t.Error("an empty name must be refused")
	}
}

func TestAPIKeyService_AuthenticateRoundTrip(t *testing.T) {
	repo := newFakeAPIKeyRepo()
	svc := NewAPIKeyService(repo)

	created, err := svc.Create(context.Background(), "tenant-a", "core", "u", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.Authenticate(context.Background(), created.Secret)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.TenantID != "tenant-a" {
		t.Errorf("resolved tenant %q, want tenant-a", got.TenantID)
	}
	if len(repo.touched) != 1 {
		t.Errorf("expected last_used_at to be recorded once, got %d", len(repo.touched))
	}
}

func TestAPIKeyService_AuthenticateRejects(t *testing.T) {
	past := time.Now().Add(-time.Hour)

	for _, tc := range []struct {
		name     string
		mutate   func(k *domain.APIKey)
		useWrong bool
	}{
		{name: "revoked", mutate: func(k *domain.APIKey) { k.RevokedAt = &past }},
		{name: "expired", mutate: func(k *domain.APIKey) { k.ExpiresAt = &past }},
		{name: "wrong secret", useWrong: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeAPIKeyRepo()
			svc := NewAPIKeyService(repo)
			created, err := svc.Create(context.Background(), "tenant-a", "core", "u", nil)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if tc.mutate != nil {
				tc.mutate(repo.created[0])
			}

			secret := created.Secret
			if tc.useWrong {
				secret = "bs_" + "not-the-right-secret"
			}

			if _, err := svc.Authenticate(context.Background(), secret); !errors.Is(err, ErrAPIKeyInvalid) {
				t.Errorf("expected ErrAPIKeyInvalid, got %v", err)
			}
		})
	}
}

// A credential that is not shaped like a key must not reach the database at
// all — every JWT-bearing request would otherwise cost a query.
func TestAPIKeyService_NonKeyCredentialSkipsLookup(t *testing.T) {
	repo := newFakeAPIKeyRepo()
	repo.lookupErr = errors.New("the repository must not be consulted")
	svc := NewAPIKeyService(repo)

	if _, err := svc.Authenticate(context.Background(), "eyJhbGciOiJIUzI1NiJ9.x.y"); !errors.Is(err, ErrAPIKeyInvalid) {
		t.Errorf("expected ErrAPIKeyInvalid for a JWT, got %v", err)
	}
}

// A database failure is not an authentication failure. Reporting it as one
// would turn an outage into "your key is invalid" — and hide the outage.
func TestAPIKeyService_LookupFailureIsNotAnAuthFailure(t *testing.T) {
	repo := newFakeAPIKeyRepo()
	repo.lookupErr = errors.New("connection refused")
	svc := NewAPIKeyService(repo)

	_, err := svc.Authenticate(context.Background(), "bs_something-plausible")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrAPIKeyInvalid) {
		t.Error("a database outage must not be reported as an invalid key")
	}
}
