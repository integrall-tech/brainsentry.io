package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

type stalenessMemoryRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Memory, error)
	Update(ctx context.Context, m *domain.Memory) error
}

type stalenessRelationshipRepository interface {
	FindByFromMemoryID(ctx context.Context, memoryID string) ([]domain.MemoryRelationship, error)
	FindByToMemoryID(ctx context.Context, memoryID string) ([]domain.MemoryRelationship, error)
}

// CascadingStalenessService propagates staleness through the knowledge graph
// when a memory is superseded, preventing obsolete data from polluting search results.
type CascadingStalenessService struct {
	memoryRepo       stalenessMemoryRepository
	relationshipRepo stalenessRelationshipRepository
	auditService     *AuditService
	maxDepth         int
}

// NewCascadingStalenessService creates a new CascadingStalenessService.
func NewCascadingStalenessService(
	memoryRepo *postgres.MemoryRepository,
	relationshipRepo *postgres.RelationshipRepository,
	auditService *AuditService,
) *CascadingStalenessService {
	return &CascadingStalenessService{
		memoryRepo:       memoryRepo,
		relationshipRepo: relationshipRepo,
		auditService:     auditService,
		maxDepth:         3,
	}
}

// StalenessResult holds the result of a staleness propagation.
type StalenessResult struct {
	SourceMemoryID  string   `json:"source_memory_id"`
	SupersededBy    string   `json:"superseded_by"`
	MarkedStale     []string `json:"marked_stale"`
	MarkedForReview []string `json:"marked_for_review"`
	Depth           int      `json:"depth"`
}

// PropagateFromSupersession propagates staleness when a memory is superseded.
func (s *CascadingStalenessService) PropagateFromSupersession(ctx context.Context, supersededID, newID string) (*StalenessResult, error) {
	result := &StalenessResult{
		SourceMemoryID: supersededID,
		SupersededBy:   newID,
	}

	visited := map[string]bool{
		supersededID: true,
		newID:        true,
	}

	currentLevel := []string{supersededID}

	for depth := 1; depth <= s.maxDepth && len(currentLevel) > 0; depth++ {
		var nextLevel []string

		for _, memID := range currentLevel {
			fromRels, err := s.relationshipRepo.FindByFromMemoryID(ctx, memID)
			if err != nil {
				slog.Warn("staleness: failed to find from-rels", "memoryId", memID, "error", err)
			}
			toRels, err2 := s.relationshipRepo.FindByToMemoryID(ctx, memID)
			if err2 != nil {
				slog.Warn("staleness: failed to find to-rels", "memoryId", memID, "error", err2)
			}

			related := append(fromRels, toRels...)
			for _, rel := range related {
				targetID := rel.ToMemoryID
				if targetID == memID {
					targetID = rel.FromMemoryID
				}

				if visited[targetID] {
					continue
				}
				visited[targetID] = true

				target, err := s.memoryRepo.FindByID(ctx, targetID)
				if err != nil || target == nil || target.DeletedAt != nil {
					continue
				}
				if target.SupersededBy != "" {
					continue
				}

				if depth == 1 {
					if err := s.setMetadataField(ctx, targetID, "staleness_source", supersededID); err != nil {
						slog.Warn("staleness: failed to mark stale", "targetId", targetID, "error", err)
						continue
					}
					result.MarkedStale = append(result.MarkedStale, targetID)
					nextLevel = append(nextLevel, targetID)
				} else {
					if err := s.setMetadataField(ctx, targetID, "needs_review_because", supersededID); err != nil {
						slog.Warn("staleness: failed to mark for review", "targetId", targetID, "error", err)
						continue
					}
					result.MarkedForReview = append(result.MarkedForReview, targetID)
				}
			}
		}

		currentLevel = nextLevel
		result.Depth = depth
	}

	if len(result.MarkedStale) > 0 || len(result.MarkedForReview) > 0 {
		slog.Info("staleness propagation completed",
			"supersededId", supersededID,
			"newId", newID,
			"markedStale", len(result.MarkedStale),
			"markedForReview", len(result.MarkedForReview),
			"depth", result.Depth,
		)

		if s.auditService != nil {
			go s.auditService.LogError(context.Background(), "staleness_propagation",
				fmt.Sprintf("superseded=%s newId=%s stale=%d review=%d",
					supersededID, newID, len(result.MarkedStale), len(result.MarkedForReview)))
		}
	}

	return result, nil
}

// setMetadataField updates a single field in the memory's JSON metadata.
func (s *CascadingStalenessService) setMetadataField(ctx context.Context, memoryID, key, value string) error {
	memory, err := s.memoryRepo.FindByID(ctx, memoryID)
	if err != nil {
		return err
	}

	// Parse existing metadata or create new
	meta := make(map[string]any)
	if len(memory.Metadata) > 0 {
		if err := json.Unmarshal(memory.Metadata, &meta); err != nil {
			meta = make(map[string]any)
		}
	}

	meta[key] = value
	meta["needs_review"] = true

	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	memory.Metadata = raw

	return s.memoryRepo.Update(ctx, memory)
}
