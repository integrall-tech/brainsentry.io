package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// RelationshipService handles memory relationship business logic.
type RelationshipService struct {
	relRepo      *postgres.RelationshipRepository
	memoryRepo   *postgres.MemoryRepository
	openRouter   *OpenRouterService
	auditService *AuditService
}

// NewRelationshipService creates a new RelationshipService.
func NewRelationshipService(
	relRepo *postgres.RelationshipRepository,
	memoryRepo *postgres.MemoryRepository,
	openRouter *OpenRouterService,
	auditService *AuditService,
) *RelationshipService {
	return &RelationshipService{
		relRepo:      relRepo,
		memoryRepo:   memoryRepo,
		openRouter:   openRouter,
		auditService: auditService,
	}
}

// CreateRelationship creates or updates a relationship between two memories.
func (s *RelationshipService) CreateRelationship(ctx context.Context, fromID, toID string, relType domain.RelationshipType) (*domain.MemoryRelationship, error) {
	tenantID := tenant.FromContext(ctx)

	// Check if relationship exists
	existing, err := s.relRepo.FindByFromAndTo(ctx, fromID, toID)
	if err == nil && existing != nil {
		existing.Frequency++
		now := time.Now()
		existing.LastUsedAt = &now
		if err := s.relRepo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	rel := &domain.MemoryRelationship{
		ID:           uuid.New().String(),
		FromMemoryID: fromID,
		ToMemoryID:   toID,
		Type:         relType,
		Frequency:    1,
		Strength:     0.5,
		CreatedAt:    time.Now(),
		TenantID:     tenantID,
	}

	if err := s.relRepo.Create(ctx, rel); err != nil {
		return nil, err
	}

	// Audit
	if s.auditService != nil {
		go s.auditService.LogRelationshipCreated(
			tenant.WithTenant(context.Background(), tenantID),
			fromID, toID, string(relType),
		)
	}

	return rel, nil
}

// CreateBidirectional creates relationships in both directions.
func (s *RelationshipService) CreateBidirectional(ctx context.Context, id1, id2 string, type1, type2 domain.RelationshipType) error {
	if _, err := s.CreateRelationship(ctx, id1, id2, type1); err != nil {
		return err
	}
	if _, err := s.CreateRelationship(ctx, id2, id1, type2); err != nil {
		return err
	}
	return nil
}

// GetRelationshipsFrom returns outgoing relationships from a memory.
func (s *RelationshipService) GetRelationshipsFrom(ctx context.Context, memoryID string) ([]domain.MemoryRelationship, error) {
	return s.relRepo.FindByFromMemoryID(ctx, memoryID)
}

// GetRelationshipsTo returns incoming relationships to a memory.
func (s *RelationshipService) GetRelationshipsTo(ctx context.Context, memoryID string) ([]domain.MemoryRelationship, error) {
	return s.relRepo.FindByToMemoryID(ctx, memoryID)
}

// GetRelationship returns a specific relationship.
func (s *RelationshipService) GetRelationship(ctx context.Context, fromID, toID string) (*domain.MemoryRelationship, error) {
	return s.relRepo.FindByFromAndTo(ctx, fromID, toID)
}

// DeleteRelationship deletes a relationship between two memories.
func (s *RelationshipService) DeleteRelationship(ctx context.Context, fromID, toID string) error {
	return s.relRepo.DeleteByFromAndTo(ctx, fromID, toID)
}

// DeleteAllForMemory deletes all relationships for a memory.
func (s *RelationshipService) DeleteAllForMemory(ctx context.Context, memoryID string) error {
	return s.relRepo.DeleteByMemoryID(ctx, memoryID)
}

// UpdateStrength updates the strength of a relationship.
func (s *RelationshipService) UpdateStrength(ctx context.Context, relationshipID string, strength float64) (*domain.MemoryRelationship, error) {
	if strength < 0 || strength > 1 {
		return nil, fmt.Errorf("strength must be between 0.0 and 1.0")
	}
	return s.relRepo.UpdateStrength(ctx, relationshipID, strength)
}

// ListAll returns all relationships for the tenant.
func (s *RelationshipService) ListAll(ctx context.Context) ([]domain.MemoryRelationship, error) {
	return s.relRepo.ListByTenant(ctx)
}

// FindRelatedMemories returns memories related to a given memory with minimum strength.
func (s *RelationshipService) FindRelatedMemories(ctx context.Context, memoryID string, minStrength float64) ([]domain.MemoryRelationship, error) {
	return s.relRepo.FindRelatedWithMinStrength(ctx, memoryID, minStrength)
}

// DetectAndCreateRelationships automatically detects relationships using LLM.
func (s *RelationshipService) DetectAndCreateRelationships(ctx context.Context, m *domain.Memory) {
	if s.openRouter == nil {
		return
	}

	tenantID := tenant.FromContext(ctx)

	// Search similar memories using the OR-of-significant-tokens variant.
	// plainto_tsquery (used by FullTextSearch) ANDs every word in the
	// query, so for a 200-char content snippet it returned zero candidates
	// for any non-trivial source — silently disabling relationship
	// suggestion. FullTextSearchSimilar extracts the top significant
	// tokens and OR's them via websearch_to_tsquery, giving a realistic
	// candidate set ranked by ts_rank.
	similar, err := s.memoryRepo.FullTextSearchSimilar(ctx, m.Content[:min(len(m.Content), 200)], 10)
	if err != nil {
		slog.Warn("failed to find similar memories for relationship detection", "error", err)
		return
	}

	for _, other := range similar {
		if other.ID == m.ID {
			continue
		}

		// LLM analysis
		analysis, err := s.openRouter.AnalyzeRelevance(ctx, m.Content, []string{other.Content})
		if err != nil {
			continue
		}

		if analysis.Relevant && analysis.Confidence >= 0.7 {
			_, err := s.CreateRelationship(
				tenant.WithTenant(context.Background(), tenantID),
				m.ID, other.ID, domain.RelRelatedTo,
			)
			if err != nil {
				slog.Warn("failed to create detected relationship", "error", err)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
