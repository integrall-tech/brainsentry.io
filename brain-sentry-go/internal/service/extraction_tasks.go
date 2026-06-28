package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/integraltech/brainsentry/pkg/tenant"
)

// Heavy extraction task types processed by the TaskScheduler. They are routed
// through the durable Redis-Streams queue (instead of a fire-and-forget
// goroutine) so a crash mid-extraction is retried/recovered rather than lost.
const (
	TaskTripletExtraction TaskType = "triplet_extraction"
	TaskEventExtraction   TaskType = "event_extraction"
)

// extractionPayload is the scheduler payload for a heavy extraction. The tenant
// travels on the AsyncTask (task.TenantID), not here, so the handler can
// reconstruct tenant context the same way the goroutine fallback does.
type extractionPayload struct {
	MemoryID string `json:"memoryId"`
	Content  string `json:"content"`
}

// ExtractionTaskHandler returns a TaskScheduler handler for the triplet and
// event extraction task types. It reconstructs tenant context from the task and
// delegates to the same runExtraction body used by the inline fallback, so the
// durable and inline paths can never diverge. Register it for BOTH
// TaskTripletExtraction and TaskEventExtraction in the composition root —
// otherwise the scheduler drops these tasks (no handler => acked and discarded).
func (s *MemoryService) ExtractionTaskHandler() TaskHandler {
	return func(ctx context.Context, task *AsyncTask) error {
		var p extractionPayload
		if err := json.Unmarshal(task.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal extraction payload: %w", err)
		}
		bgCtx := tenant.WithTenant(ctx, task.TenantID)
		return s.runExtraction(bgCtx, task.Type, p.MemoryID, p.Content)
	}
}
