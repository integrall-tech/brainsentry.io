package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// EventRepository persists Event records.
type EventRepository struct {
	pool *pgxpool.Pool
}

// NewEventRepository constructs the repository.
func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

// eventColumns is the INSERT column list — bare names, no casts.
const eventColumns = `id, tenant_id, event_type, title, description, occurred_at,
	participants, attributes, source_memory_id, embedding, created_at`

// eventSelectColumns casts embedding back to float4[] so pgx can scan it into
// []float32 — pgvector's own text output is not scannable. See vector.go.
var eventSelectColumns = `id, tenant_id, event_type, title, description, occurred_at,
	participants, attributes, source_memory_id, ` + vectorSelect("embedding") + `, created_at`

func scanEvent(row pgx.Row) (*domain.Event, error) {
	var e domain.Event
	var participantsRaw []byte
	var sourceMemoryID *string
	if err := row.Scan(&e.ID, &e.TenantID, &e.EventType, &e.Title, &e.Description,
		&e.OccurredAt, &participantsRaw, &e.Attributes, &sourceMemoryID, &e.Embedding, &e.CreatedAt); err != nil {
		return nil, err
	}
	if sourceMemoryID != nil {
		e.SourceMemoryID = *sourceMemoryID
	}
	_ = json.Unmarshal(participantsRaw, &e.Participants)
	return &e, nil
}

// Create inserts a new event.
func (r *EventRepository) Create(ctx context.Context, e *domain.Event) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.TenantID == "" {
		e.TenantID = tenant.FromContext(ctx)
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = e.CreatedAt
	}
	participants, _ := json.Marshal(e.Participants)
	if len(participants) == 0 {
		participants = []byte("[]")
	}
	attrs := e.Attributes
	if len(attrs) == 0 {
		attrs = []byte("{}")
	}
	var sourceMemoryID *string
	if e.SourceMemoryID != "" {
		sourceMemoryID = &e.SourceMemoryID
	}

	// $10::vector — see the note in vector.go on why the cast is required.
	query := fmt.Sprintf(`INSERT INTO events (%s) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::vector,$11)`, eventColumns)
	_, err := r.pool.Exec(ctx, query,
		e.ID, e.TenantID, e.EventType, e.Title, e.Description, e.OccurredAt,
		participants, attrs, sourceMemoryID, vectorParam(e.Embedding), e.CreatedAt)
	return err
}

// FindByID returns a single event.
func (r *EventRepository) FindByID(ctx context.Context, id string) (*domain.Event, error) {
	tenantID := tenant.FromContext(ctx)
	query := fmt.Sprintf(`SELECT %s FROM events WHERE id=$1 AND tenant_id=$2`, eventSelectColumns)
	e, err := scanEvent(r.pool.QueryRow(ctx, query, id, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("event not found: %s", id)
		}
		return nil, err
	}
	return e, nil
}

// EventFilter narrows List queries.
type EventFilter struct {
	EventType   string
	EntityID    string
	FromTime    *time.Time
	ToTime      *time.Time
	Limit       int
}

// List returns events matching the filter.
func (r *EventRepository) List(ctx context.Context, f EventFilter) ([]*domain.Event, error) {
	tenantID := tenant.FromContext(ctx)
	args := []any{tenantID}
	where := "tenant_id = $1"
	if f.EventType != "" {
		args = append(args, f.EventType)
		where += fmt.Sprintf(" AND event_type = $%d", len(args))
	}
	if f.EntityID != "" {
		args = append(args, f.EntityID)
		where += fmt.Sprintf(" AND participants @> jsonb_build_array(jsonb_build_object('entityId', $%d::text))", len(args))
	}
	if f.FromTime != nil {
		args = append(args, *f.FromTime)
		where += fmt.Sprintf(" AND occurred_at >= $%d", len(args))
	}
	if f.ToTime != nil {
		args = append(args, *f.ToTime)
		where += fmt.Sprintf(" AND occurred_at <= $%d", len(args))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf(`SELECT %s FROM events WHERE %s ORDER BY occurred_at DESC LIMIT %d`,
		eventSelectColumns, where, limit)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountByType returns event counts per event_type.
func (r *EventRepository) CountByType(ctx context.Context) (map[string]int, error) {
	tenantID := tenant.FromContext(ctx)
	rows, err := r.pool.Query(ctx,
		`SELECT event_type, COUNT(*) FROM events WHERE tenant_id=$1 GROUP BY event_type ORDER BY 2 DESC`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}

// Delete removes an event.
func (r *EventRepository) Delete(ctx context.Context, id string) error {
	tenantID := tenant.FromContext(ctx)
	_, err := r.pool.Exec(ctx, `DELETE FROM events WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	return err
}
