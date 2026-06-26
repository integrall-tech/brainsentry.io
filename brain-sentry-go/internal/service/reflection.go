package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// ReflectionService performs automatic reflection loops over accumulated memories.
// Clusters similar memories → computes saliency → synthesizes reflective summaries.
type ReflectionService struct {
	openRouter    *OpenRouterService
	memoryRepo    *postgres.MemoryRepository
	memoryService *MemoryService
	minClusterSize int
	similarityThreshold float64 // SimHash Hamming distance threshold for clustering
}

// NewReflectionService creates a new ReflectionService.
func NewReflectionService(
	openRouter *OpenRouterService,
	memoryRepo *postgres.MemoryRepository,
	memoryService *MemoryService,
) *ReflectionService {
	return &ReflectionService{
		openRouter:          openRouter,
		memoryRepo:          memoryRepo,
		memoryService:       memoryService,
		minClusterSize:      3,
		similarityThreshold: 8, // Hamming distance ≤ 8
	}
}

// MemoryCluster represents a group of similar memories.
type MemoryCluster struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Memories  []domain.Memory `json:"memories"`
	Saliency  float64         `json:"saliency"` // aggregate importance
	Size      int             `json:"size"`
}

// ReflectionResult summarizes what the reflection loop produced.
type ReflectionResult struct {
	ClustersFound     int      `json:"clustersFound"`
	ReflectionsCreated int     `json:"reflectionsCreated"`
	MemoriesProcessed int      `json:"memoriesProcessed"`
	ConsolidatedIDs   []string `json:"consolidatedIds"`
}

// RunReflection performs one reflection cycle: cluster → score → synthesize.
func (s *ReflectionService) RunReflection(ctx context.Context) (*ReflectionResult, error) {
	result := &ReflectionResult{}

	// Step 1: Fetch all active memories with SimHash
	simHashes, err := s.memoryRepo.FindSimHashes(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching sim hashes: %w", err)
	}

	if len(simHashes) < s.minClusterSize {
		return result, nil // not enough memories to cluster
	}

	// Step 2: Cluster by SimHash proximity
	clusters := s.clusterBySimHash(ctx, simHashes)
	result.ClustersFound = len(clusters)

	if len(clusters) == 0 {
		return result, nil
	}

	// Step 3: For each cluster, compute saliency and synthesize
	for _, cluster := range clusters {
		result.MemoriesProcessed += cluster.Size

		// Compute cluster saliency
		cluster.Saliency = s.computeClusterSaliency(cluster)

		// Only synthesize meaningful clusters
		if cluster.Saliency < 0.3 {
			continue
		}

		// Step 4: Synthesize reflective memory
		reflection, err := s.synthesizeReflection(ctx, cluster)
		if err != nil {
			slog.Warn("failed to synthesize reflection", "error", err, "cluster", cluster.ID)
			continue
		}

		if reflection != "" {
			tenantID := tenant.FromContext(ctx)
			go func(content string, clusterMemories []domain.Memory) {
				bgCtx := tenant.WithTenant(context.Background(), tenantID)
				sourceIDs := make([]string, 0, len(clusterMemories))
				for _, m := range clusterMemories {
					sourceIDs = append(sourceIDs, m.ID)
				}

				_, err := s.memoryService.CreateMemory(bgCtx, dto.CreateMemoryRequest{
					Content:    content,
					SourceType: "reflection",
					Metadata: map[string]any{
						"source":         "auto_reflection",
						"consolidatedFrom": sourceIDs,
						"clusterSize":    len(clusterMemories),
					},
				})
				if err != nil {
					slog.Warn("failed to create reflective memory", "error", err)
					return
				}

				// Mark source memories as consolidated
				for _, m := range clusterMemories {
					meta := make(map[string]any)
					if m.Metadata != nil {
						json.Unmarshal(m.Metadata, &meta)
					}
					meta["consolidated"] = true
					meta["consolidatedAt"] = time.Now().Format(time.RFC3339)
					if _, err := s.memoryService.UpdateMemory(bgCtx, m.ID, dto.UpdateMemoryRequest{
						Metadata:     meta,
						ChangeReason: "auto-reflection consolidation",
					}); err != nil {
						slog.Warn("reflection: failed to mark memory consolidated", "memoryId", m.ID, "error", err)
					}
				}
			}(reflection, cluster.Memories)

			result.ReflectionsCreated++
			for _, m := range cluster.Memories {
				result.ConsolidatedIDs = append(result.ConsolidatedIDs, m.ID)
			}
		}
	}

	return result, nil
}

func (s *ReflectionService) clusterBySimHash(ctx context.Context, simHashes map[string]string) []MemoryCluster {
	type idHash struct {
		id   string
		hash uint64
	}

	// Convert to list
	var entries []idHash
	for id, hex := range simHashes {
		entries = append(entries, idHash{id: id, hash: SimHashFromHex(hex)})
	}

	// Simple greedy clustering by Hamming distance
	used := make(map[string]bool)
	var clusters []MemoryCluster
	clusterNum := 0

	for i := 0; i < len(entries); i++ {
		if used[entries[i].id] {
			continue
		}

		cluster := MemoryCluster{
			ID: fmt.Sprintf("cluster-%d", clusterNum),
		}
		clusterIDs := []string{entries[i].id}
		used[entries[i].id] = true

		for j := i + 1; j < len(entries); j++ {
			if used[entries[j].id] {
				continue
			}
			dist := SimHashHammingDistance(entries[i].hash, entries[j].hash)
			if dist <= int(s.similarityThreshold) {
				clusterIDs = append(clusterIDs, entries[j].id)
				used[entries[j].id] = true
			}
		}

		if len(clusterIDs) < s.minClusterSize {
			continue
		}

		// Load full memories for cluster
		for _, id := range clusterIDs {
			m, err := s.memoryRepo.FindByID(ctx, id)
			if err == nil && !IsExpired(m, time.Now()) && m.SupersededBy == "" {
				cluster.Memories = append(cluster.Memories, *m)
			}
		}

		if len(cluster.Memories) >= s.minClusterSize {
			cluster.Size = len(cluster.Memories)
			clusters = append(clusters, cluster)
			clusterNum++
		}
	}

	return clusters
}

func (s *ReflectionService) computeClusterSaliency(cluster MemoryCluster) float64 {
	if cluster.Size == 0 {
		return 0
	}

	totalDecayed := 0.0
	now := time.Now()
	for _, m := range cluster.Memories {
		totalDecayed += ComputeDecayedRelevance(&m, now)
	}

	// Normalize by cluster size, boosted by size itself (larger clusters = more important)
	avgDecayed := totalDecayed / float64(cluster.Size)
	sizeBoost := 1.0 + 0.1*float64(cluster.Size)

	return avgDecayed * sizeBoost
}

func (s *ReflectionService) synthesizeReflection(ctx context.Context, cluster MemoryCluster) (string, error) {
	if s.openRouter == nil {
		// Without LLM, create a simple concatenation summary
		return s.simpleSynthesis(cluster), nil
	}

	var memorySummaries string
	for i, m := range cluster.Memories {
		summary := m.Summary
		if summary == "" {
			summary = truncate(m.Content, 200)
		}
		memorySummaries += fmt.Sprintf("[%d] [%s] %s\n", i+1, m.MemoryType, summary)
	}

	prompt := fmt.Sprintf(`The following %d memories are clustered as similar/related. Synthesize them into a single reflective insight that captures the higher-order pattern, lesson, or principle they collectively represent.

RULES:
- Write a concise meta-observation (2-4 sentences)
- Focus on the "why" and patterns, not individual facts
- Use full names, no pronouns
- The reflection should be more valuable than any individual memory

Memories:
%s

Respond with just the reflective insight text, no JSON needed.`, cluster.Size, memorySummaries)

	response, err := s.openRouter.Chat(ctx, []ChatMessage{
		{Role: "system", Content: "You are a reflection system. Synthesize groups of similar memories into higher-order insights and patterns."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return s.simpleSynthesis(cluster), nil
	}

	return strings.TrimSpace(response), nil
}

func (s *ReflectionService) simpleSynthesis(cluster MemoryCluster) string {
	var topics []string
	for _, m := range cluster.Memories {
		if m.Summary != "" {
			topics = append(topics, m.Summary)
		} else {
			topics = append(topics, truncate(m.Content, 100))
		}
	}

	return fmt.Sprintf("[Auto-reflection] Recurring theme across %d memories: %s",
		cluster.Size, joinStrings(topics, "; "))
}
