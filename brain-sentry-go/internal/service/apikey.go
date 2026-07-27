package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

// ErrAPIKeyInvalid is returned for every authentication failure — unknown
// prefix, wrong secret, revoked, expired.
//
// One error for all of them on purpose: telling a caller "this key exists but
// is revoked" versus "no such key" is a probe for enumerating valid keys.
var ErrAPIKeyInvalid = errors.New("invalid api key")

// APIKeyRepo is the persistence contract for service keys.
type APIKeyRepo interface {
	Create(ctx context.Context, k *domain.APIKey) error
	FindCandidatesByPrefix(ctx context.Context, prefix string) ([]*domain.APIKey, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*domain.APIKey, error)
	FindByID(ctx context.Context, id string) (*domain.APIKey, error)
	Revoke(ctx context.Context, id string) error
	TouchLastUsed(ctx context.Context, id string) error
}

// APIKeyService issues and authenticates service keys.
type APIKeyService struct {
	repo APIKeyRepo
}

// NewAPIKeyService creates a new APIKeyService.
func NewAPIKeyService(repo APIKeyRepo) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// CreatedAPIKey carries the one and only look at the plaintext secret.
type CreatedAPIKey struct {
	Key *domain.APIKey
	// Secret is returned once, at creation, and never retrievable again —
	// only its hash is stored.
	Secret string
}

// Create issues a new key for a tenant.
//
// tenantID comes from the route, which is admin-only. There is deliberately
// no "current tenant" default here: issuing a credential is exactly the
// operation where an implicit tenant would be dangerous.
func (s *APIKeyService) Create(ctx context.Context, tenantID, name, createdBy string, expiresAt *time.Time) (*CreatedAPIKey, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenantId is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	secret, prefix, err := domain.GenerateAPIKeySecret()
	if err != nil {
		return nil, err
	}

	key := &domain.APIKey{
		TenantID:  tenantID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   domain.HashAPIKeySecret(secret),
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, err
	}

	slog.Info("api key created",
		"keyId", key.ID, "tenantId", tenantID, "name", name, "createdBy", createdBy)

	return &CreatedAPIKey{Key: key, Secret: secret}, nil
}

// Authenticate resolves a presented secret to its key.
//
// The tenant comes from the stored row — never from anything the caller sent.
// That is the whole security property of this credential: a request cannot
// name the tenant it wants to act on.
func (s *APIKeyService) Authenticate(ctx context.Context, secret string) (*domain.APIKey, error) {
	if !domain.LooksLikeAPIKey(secret) {
		return nil, ErrAPIKeyInvalid
	}

	candidates, err := s.repo.FindCandidatesByPrefix(ctx, domain.APIKeyLookupPrefix(secret))
	if err != nil {
		// A database failure is not an authentication failure, and must not
		// be reported as one: fail closed, but say why in the log.
		return nil, fmt.Errorf("looking up api key: %w", err)
	}

	now := time.Now()
	for _, k := range candidates {
		if !domain.APIKeySecretMatches(secret, k.KeyHash) {
			continue
		}
		if !k.IsUsable(now) {
			// Matched, but revoked or expired. Same error as "not found" —
			// see ErrAPIKeyInvalid.
			return nil, ErrAPIKeyInvalid
		}
		// Best-effort bookkeeping; a failure here must not fail the request.
		if err := s.repo.TouchLastUsed(ctx, k.ID); err != nil {
			slog.Warn("could not update api key last_used_at", "keyId", k.ID, "error", err)
		}
		return k, nil
	}
	return nil, ErrAPIKeyInvalid
}

// List returns a tenant's keys. Secrets are not recoverable, so this is safe
// to expose to an admin surface.
func (s *APIKeyService) List(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

// Revoke invalidates a key immediately.
func (s *APIKeyService) Revoke(ctx context.Context, keyID string) error {
	key, err := s.repo.FindByID(ctx, keyID)
	if err != nil {
		return err
	}
	if err := s.repo.Revoke(ctx, keyID); err != nil {
		return err
	}
	slog.Info("api key revoked", "keyId", keyID, "tenantId", key.TenantID)
	return nil
}

// Get returns a key by id (without its secret, which is not stored).
func (s *APIKeyService) Get(ctx context.Context, keyID string) (*domain.APIKey, error) {
	return s.repo.FindByID(ctx, keyID)
}
