package graph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

// MemoryGraphRepository handles memory operations in FalkorDB.
type MemoryGraphRepository struct {
	client *Client
}

// NewMemoryGraphRepository creates a new MemoryGraphRepository.
func NewMemoryGraphRepository(client *Client) *MemoryGraphRepository {
	return &MemoryGraphRepository{client: client}
}

// DropGraph wipes the entire FalkorDB graph. Operator-only — wired into
// the rebuild executor (internal/rebuild). See the system-of-record doc
// for context.
func (r *MemoryGraphRepository) DropGraph(ctx context.Context) error {
	return r.client.DropGraph(ctx)
}

// SaveToGraph stores or updates a memory node in the graph.
func (r *MemoryGraphRepository) SaveToGraph(ctx context.Context, m *domain.Memory) error {
	tagsStr := formatStringList(m.Tags)
	embeddingStr := formatFloatList(m.Embedding)

	cypher := fmt.Sprintf(`MERGE (m:Memory {id: '%s'})
SET m.content = '%s',
    m.summary = '%s',
    m.category = '%s',
    m.importance = '%s',
    m.tenantId = '%s',
    m.tags = %s,
    m.embedding = %s,
    m.createdAt = %d,
    m.accessCount = %d,
    m.version = %d`,
		EscapeCypher(m.ID),
		EscapeCypher(m.Content),
		EscapeCypher(m.Summary),
		EscapeCypher(string(m.Category)),
		EscapeCypher(string(m.Importance)),
		EscapeCypher(m.TenantID),
		tagsStr,
		embeddingStr,
		m.CreatedAt.UnixMilli(),
		m.AccessCount,
		m.Version,
	)

	_, err := r.client.Query(ctx, cypher)
	if err != nil {
		return fmt.Errorf("saving memory to graph: %w", err)
	}

	return nil
}

// CreateTagRelationships creates RELATED_TO edges between memories that share tags.
func (r *MemoryGraphRepository) CreateTagRelationships(ctx context.Context, m *domain.Memory) error {
	if len(m.Tags) == 0 {
		return nil
	}

	for _, tag := range m.Tags {
		cypher := fmt.Sprintf(`MATCH (m1:Memory {id: '%s'}), (m2:Memory)
WHERE m2.tenantId = '%s' AND m1.id <> m2.id AND '%s' IN m2.tags
MERGE (m1)-[r:RELATED_TO]->(m2)
ON CREATE SET r.type = 'shared_tag', r.tag = '%s', r.strength = 1, r.mentions = 1, r.updatedAt = %d
ON MATCH SET r.strength = r.strength + 1, r.mentions = coalesce(r.mentions, 0) + 1, r.updatedAt = %d`,
			EscapeCypher(m.ID),
			EscapeCypher(m.TenantID),
			EscapeCypher(tag),
			EscapeCypher(tag),
			time.Now().UnixMilli(),
			time.Now().UnixMilli(),
		)

		if _, err := r.client.Query(ctx, cypher); err != nil {
			slog.Warn("failed to create tag relationship", "error", err, "tag", tag)
		}
	}

	return nil
}

// CreateAllRelationships batch creates relationships for all memories in a tenant.
func (r *MemoryGraphRepository) CreateAllRelationships(ctx context.Context, tenantID string) error {
	cypher := fmt.Sprintf(`MATCH (m1:Memory) WHERE m1.tenantId = '%s'
UNWIND m1.tags AS tag1
MATCH (m2:Memory) WHERE m2.tenantId = '%s' AND m1.id < m2.id AND tag1 IN m2.tags
WITH m1, m2, collect(DISTINCT tag1)[0] as sharedTag
MERGE (m1)-[r:RELATED_TO]->(m2)
ON CREATE SET r.type = 'shared_tag', r.tag = sharedTag, r.strength = 1, r.mentions = 1, r.updatedAt = %d
ON MATCH SET r.strength = r.strength + 1, r.mentions = coalesce(r.mentions, 0) + 1, r.updatedAt = %d`,
		EscapeCypher(tenantID),
		EscapeCypher(tenantID),
		time.Now().UnixMilli(),
		time.Now().UnixMilli(),
	)

	_, err := r.client.Query(ctx, cypher)
	return err
}

// VectorSearch performs vector similarity search in the graph.
func (r *MemoryGraphRepository) VectorSearch(ctx context.Context, embedding []float32, limit int, tenantID string) ([]string, []float64, error) {
	embeddingStr := formatFloatList(embedding)

	// Try vector search first
	cypher := fmt.Sprintf(`CALL db.idx.vector.queryNodes('Memory', 'embedding', %d, %s)
YIELD node, score
WHERE node.tenantId = '%s'
RETURN node.id as id, score
LIMIT %d`,
		limit,
		embeddingStr,
		EscapeCypher(tenantID),
		limit,
	)

	result, err := r.client.Query(ctx, cypher)
	if err != nil {
		slog.Warn("vector search failed, falling back to access-based", "error", err)
		return r.fallbackSearch(ctx, limit, tenantID)
	}

	if len(result.Records) == 0 {
		return r.fallbackSearch(ctx, limit, tenantID)
	}

	ids := make([]string, 0, len(result.Records))
	scores := make([]float64, 0, len(result.Records))
	for _, rec := range result.Records {
		ids = append(ids, GetString(rec.Values, "id"))
		scores = append(scores, GetFloat64(rec.Values, "score"))
	}

	return ids, scores, nil
}

// fallbackSearch returns most-accessed memories when vector search fails.
func (r *MemoryGraphRepository) fallbackSearch(ctx context.Context, limit int, tenantID string) ([]string, []float64, error) {
	cypher := fmt.Sprintf(`MATCH (m:Memory)
WHERE m.tenantId = '%s'
RETURN m.id as id, m.accessCount as score
ORDER BY m.accessCount DESC
LIMIT %d`,
		EscapeCypher(tenantID),
		limit,
	)

	result, err := r.client.Query(ctx, cypher)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]string, 0, len(result.Records))
	scores := make([]float64, 0, len(result.Records))
	for _, rec := range result.Records {
		ids = append(ids, GetString(rec.Values, "id"))
		scores = append(scores, GetFloat64(rec.Values, "score"))
	}

	return ids, scores, nil
}

// FindRelated finds related memories by graph traversal.
func (r *MemoryGraphRepository) FindRelated(ctx context.Context, memoryID string, depth int, tenantID string) ([]string, error) {
	if depth <= 0 {
		depth = 2
	}

	cypher := fmt.Sprintf(`MATCH (m:Memory {id: '%s'})-[r:RELATED_TO*1..%d]-(related:Memory)
WHERE related.tenantId = '%s'
RETURN DISTINCT related.id as id, count(r) as relationshipCount
ORDER BY relationshipCount DESC
LIMIT %d`,
		EscapeCypher(memoryID),
		depth,
		EscapeCypher(tenantID),
		depth*5,
	)

	result, err := r.client.Query(ctx, cypher)
	if err != nil {
		return nil, fmt.Errorf("finding related: %w", err)
	}

	ids := make([]string, 0, len(result.Records))
	for _, rec := range result.Records {
		ids = append(ids, GetString(rec.Values, "id"))
	}

	return ids, nil
}

// GetGraphRelationships returns all RELATED_TO edges for a tenant.
func (r *MemoryGraphRepository) GetGraphRelationships(ctx context.Context, tenantID string, limit int) ([]GraphRelationship, error) {
	if limit <= 0 {
		limit = 100
	}

	cypher := fmt.Sprintf(`MATCH (m1:Memory)-[r:RELATED_TO]->(m2:Memory)
WHERE m1.tenantId = '%s'
RETURN m1.id as fromId, m2.id as toId, m1.summary as fromSummary, m2.summary as toSummary,
       r.tag as tag, r.strength as strength, r.type as type
LIMIT %d`,
		EscapeCypher(tenantID),
		limit,
	)

	result, err := r.client.Query(ctx, cypher)
	if err != nil {
		return nil, fmt.Errorf("getting graph relationships: %w", err)
	}

	rels := make([]GraphRelationship, 0, len(result.Records))
	for _, rec := range result.Records {
		rels = append(rels, GraphRelationship{
			FromID:      GetString(rec.Values, "fromId"),
			ToID:        GetString(rec.Values, "toId"),
			FromSummary: GetString(rec.Values, "fromSummary"),
			ToSummary:   GetString(rec.Values, "toSummary"),
			Tag:         GetString(rec.Values, "tag"),
			Strength:    GetFloat64(rec.Values, "strength"),
			Type:        GetString(rec.Values, "type"),
		})
	}

	return rels, nil
}

// DeleteMemory removes a memory node and its edges from the graph.
func (r *MemoryGraphRepository) DeleteMemory(ctx context.Context, memoryID string) error {
	cypher := fmt.Sprintf(`MATCH (m:Memory {id: '%s'}) DETACH DELETE m`, EscapeCypher(memoryID))
	_, err := r.client.Query(ctx, cypher)
	return err
}

// GraphRelationship represents a relationship returned from the graph.
type GraphRelationship struct {
	FromID      string  `json:"fromId"`
	ToID        string  `json:"toId"`
	FromSummary string  `json:"fromSummary"`
	ToSummary   string  `json:"toSummary"`
	Tag         string  `json:"tag"`
	Strength    float64 `json:"strength"`
	Type        string  `json:"type"`
}

func formatStringList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	escaped := make([]string, len(items))
	for i, item := range items {
		escaped[i] = "'" + EscapeCypher(item) + "'"
	}
	return "[" + strings.Join(escaped, ", ") + "]"
}

func formatFloatList(values []float32) string {
	if len(values) == 0 {
		return "[]"
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
