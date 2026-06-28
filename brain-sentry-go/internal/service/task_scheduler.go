package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// TaskType represents the type of async task.
type TaskType string

const (
	TaskEntityExtraction TaskType = "entity_extraction"
	TaskSummarization    TaskType = "summarization"
	TaskReflection       TaskType = "reflection"
	TaskReconciliation   TaskType = "reconciliation"
	TaskEmbedding        TaskType = "embedding"
	TaskGraphUpdate      TaskType = "graph_update"
	TaskDecayCleanup     TaskType = "decay_cleanup"
	TaskProfileUpdate    TaskType = "profile_update"
)

// TaskStatus represents the lifecycle status of an async task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusRetrying   TaskStatus = "retrying"
)

// TaskPriority controls processing order.
type TaskPriority int

const (
	PriorityLow    TaskPriority = 1
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 10
	PriorityCritical TaskPriority = 20
)

// AsyncTask represents a task to be processed asynchronously.
type AsyncTask struct {
	ID         string       `json:"id"`
	Type       TaskType     `json:"type"`
	TenantID   string       `json:"tenantId"`
	UserID     string       `json:"userId,omitempty"`
	Priority   TaskPriority `json:"priority"`
	Payload    json.RawMessage `json:"payload"`
	Status     TaskStatus   `json:"status"`
	Attempts   int          `json:"attempts"`
	MaxRetries int          `json:"maxRetries"`
	Error      string       `json:"error,omitempty"`
	CreatedAt  time.Time    `json:"createdAt"`
	StartedAt  *time.Time   `json:"startedAt,omitempty"`
	CompletedAt *time.Time  `json:"completedAt,omitempty"`
}

// TaskHandler processes a specific task type.
type TaskHandler func(ctx context.Context, task *AsyncTask) error

// TaskSchedulerConfig holds scheduler configuration.
type TaskSchedulerConfig struct {
	StreamName       string        // Redis stream name
	GroupName        string        // Consumer group name
	ConsumerName     string        // Consumer name
	MaxRetries       int           // Default max retries per task
	ClaimTimeout     time.Duration // Time before unclaimed tasks are reclaimed
	PollInterval     time.Duration // Polling interval for new tasks
	WorkerCount      int           // Number of concurrent workers
}

// DefaultTaskSchedulerConfig returns sensible defaults.
func DefaultTaskSchedulerConfig() TaskSchedulerConfig {
	return TaskSchedulerConfig{
		StreamName:   "brainsentry:tasks",
		GroupName:    "workers",
		ConsumerName: "worker-1",
		MaxRetries:   3,
		ClaimTimeout: 5 * time.Minute,
		PollInterval: 1 * time.Second,
		WorkerCount:  3,
	}
}

// TaskScheduler manages async task processing via Redis Streams.
type TaskScheduler struct {
	client   *redis.Client
	config   TaskSchedulerConfig
	handlers map[TaskType]TaskHandler
	mu       sync.RWMutex
	stopCh   chan struct{}
	wg       sync.WaitGroup

	// Per-tenant priority weights
	tenantWeights map[string]float64
	weightsMu     sync.RWMutex

	// Metrics
	processed  int64
	failed     int64
	recovered  int64
	metricsMu  sync.Mutex
}

// NewTaskScheduler creates a new TaskScheduler.
func NewTaskScheduler(client *redis.Client, config TaskSchedulerConfig) *TaskScheduler {
	return &TaskScheduler{
		client:        client,
		config:        config,
		handlers:      make(map[TaskType]TaskHandler),
		stopCh:        make(chan struct{}),
		tenantWeights: make(map[string]float64),
	}
}

// RegisterHandler registers a handler for a specific task type.
func (s *TaskScheduler) RegisterHandler(taskType TaskType, handler TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[taskType] = handler
}

// SetTenantWeight sets the priority weight for a tenant (default: 1.0).
func (s *TaskScheduler) SetTenantWeight(tenantID string, weight float64) {
	s.weightsMu.Lock()
	defer s.weightsMu.Unlock()
	s.tenantWeights[tenantID] = weight
}

// GetTenantWeight returns the priority weight for a tenant.
func (s *TaskScheduler) GetTenantWeight(tenantID string) float64 {
	s.weightsMu.RLock()
	defer s.weightsMu.RUnlock()
	w, ok := s.tenantWeights[tenantID]
	if !ok {
		return 1.0
	}
	return w
}

// Submit adds a new task to the queue.
func (s *TaskScheduler) Submit(ctx context.Context, taskType TaskType, tenantID, userID string, priority TaskPriority, payload any) (*AsyncTask, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload: %w", err)
	}

	task := &AsyncTask{
		ID:         uuid.New().String(),
		Type:       taskType,
		TenantID:   tenantID,
		UserID:     userID,
		Priority:   priority,
		Payload:    payloadJSON,
		Status:     TaskStatusPending,
		MaxRetries: s.config.MaxRetries,
		CreatedAt:  time.Now(),
	}

	// Apply tenant weight to effective priority
	weight := s.GetTenantWeight(tenantID)
	effectivePriority := float64(priority) * weight

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshaling task: %w", err)
	}

	if s.client == nil {
		// In-memory mode: process synchronously
		return task, s.processTaskInline(ctx, task)
	}

	// Add to Redis stream with priority field
	_, err = s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.config.StreamName,
		Values: map[string]interface{}{
			"task":     string(taskJSON),
			"priority": fmt.Sprintf("%.1f", effectivePriority),
			"type":     string(taskType),
			"tenant":   tenantID,
		},
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("adding to stream: %w", err)
	}

	slog.Debug("task submitted", "id", task.ID, "type", taskType, "priority", effectivePriority)
	return task, nil
}

// Start begins processing tasks from the stream.
func (s *TaskScheduler) Start(ctx context.Context) error {
	if s.client == nil {
		return nil
	}

	// Create consumer group. BUSYGROUP (already exists) is expected and
	// benign; anything else is a real setup failure worth surfacing.
	if err := s.client.XGroupCreateMkStream(ctx, s.config.StreamName, s.config.GroupName, "0").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create consumer group: %w", err)
	}

	// Start workers
	for i := 0; i < s.config.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// Start recovery goroutine
	s.wg.Add(1)
	go s.recoveryWorker(ctx)

	slog.Info("task scheduler started",
		"workers", s.config.WorkerCount,
		"stream", s.config.StreamName,
	)
	return nil
}

// Stop gracefully stops the scheduler.
func (s *TaskScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	slog.Info("task scheduler stopped")
}

// Metrics returns current scheduler metrics.
func (s *TaskScheduler) Metrics() (processed, failed, recovered int64) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	return s.processed, s.failed, s.recovered
}

func (s *TaskScheduler) worker(ctx context.Context, workerID int) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		// Read from stream
		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    s.config.GroupName,
			Consumer: fmt.Sprintf("%s-%d", s.config.ConsumerName, workerID),
			Streams:  []string{s.config.StreamName, ">"},
			Count:    1,
			Block:    s.config.PollInterval,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue
			}
			select {
			case <-s.stopCh:
				return
			default:
				slog.Debug("stream read error", "worker", workerID, "error", err)
				time.Sleep(s.config.PollInterval)
				continue
			}
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				s.processMessage(ctx, msg)
			}
		}
	}
}

func (s *TaskScheduler) processMessage(ctx context.Context, msg redis.XMessage) {
	taskJSON, ok := msg.Values["task"].(string)
	if !ok {
		slog.Warn("invalid task message", "id", msg.ID)
		s.ackMessage(ctx, msg.ID)
		return
	}

	var task AsyncTask
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		slog.Warn("failed to unmarshal task", "error", err)
		s.ackMessage(ctx, msg.ID)
		return
	}

	task.Status = TaskStatusProcessing
	now := time.Now()
	task.StartedAt = &now
	task.Attempts++

	// Get handler
	s.mu.RLock()
	handler, exists := s.handlers[task.Type]
	s.mu.RUnlock()

	if !exists {
		slog.Warn("no handler for task type", "type", task.Type)
		s.ackMessage(ctx, msg.ID)
		return
	}

	// Execute handler
	if err := handler(ctx, &task); err != nil {
		task.Error = err.Error()

		if task.Attempts < task.MaxRetries {
			task.Status = TaskStatusRetrying
			slog.Warn("task failed, will retry",
				"id", task.ID, "type", task.Type,
				"attempt", task.Attempts, "error", err,
			)
			// Re-submit for retry
			s.resubmit(ctx, &task)
		} else {
			task.Status = TaskStatusFailed
			slog.Error("task failed permanently",
				"id", task.ID, "type", task.Type,
				"attempts", task.Attempts, "error", err,
			)
			s.metricsMu.Lock()
			s.failed++
			s.metricsMu.Unlock()
		}
	} else {
		completed := time.Now()
		task.Status = TaskStatusCompleted
		task.CompletedAt = &completed

		s.metricsMu.Lock()
		s.processed++
		s.metricsMu.Unlock()

		slog.Debug("task completed",
			"id", task.ID, "type", task.Type,
			"duration", completed.Sub(now),
		)
	}

	s.ackMessage(ctx, msg.ID)
}

func (s *TaskScheduler) ackMessage(ctx context.Context, msgID string) {
	s.client.XAck(ctx, s.config.StreamName, s.config.GroupName, msgID)
}

func (s *TaskScheduler) resubmit(ctx context.Context, task *AsyncTask) {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		slog.Warn("failed to resubmit task", "error", err)
		return
	}

	s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.config.StreamName,
		Values: map[string]interface{}{
			"task":     string(taskJSON),
			"priority": fmt.Sprintf("%d", task.Priority),
			"type":     string(task.Type),
			"tenant":   task.TenantID,
		},
	})
}

// recoveryWorker periodically reclaims stuck tasks.
func (s *TaskScheduler) recoveryWorker(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.ClaimTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.recoverStuckTasks(ctx)
		}
	}
}

func (s *TaskScheduler) recoverStuckTasks(ctx context.Context) {
	if s.client == nil {
		return
	}

	// Find pending messages older than ClaimTimeout
	pending, err := s.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: s.config.StreamName,
		Group:  s.config.GroupName,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()

	if err != nil {
		return
	}

	for _, p := range pending {
		if p.Idle < s.config.ClaimTimeout {
			continue
		}

		// Claim the stuck message
		claimed, err := s.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   s.config.StreamName,
			Group:    s.config.GroupName,
			Consumer: s.config.ConsumerName,
			MinIdle:  s.config.ClaimTimeout,
			Messages: []string{p.ID},
		}).Result()

		if err != nil {
			slog.Warn("failed to claim stuck task", "id", p.ID, "error", err)
			continue
		}

		for _, msg := range claimed {
			slog.Info("recovered stuck task", "id", msg.ID, "idle", p.Idle)
			s.processMessage(ctx, msg)
			s.metricsMu.Lock()
			s.recovered++
			s.metricsMu.Unlock()
		}
	}
}

// processTaskInline processes a task synchronously when no Redis is available.
func (s *TaskScheduler) processTaskInline(ctx context.Context, task *AsyncTask) error {
	s.mu.RLock()
	handler, exists := s.handlers[task.Type]
	s.mu.RUnlock()

	if !exists {
		return nil // no handler, skip silently
	}

	task.Status = TaskStatusProcessing
	now := time.Now()
	task.StartedAt = &now
	task.Attempts = 1

	if err := handler(ctx, task); err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		return err
	}

	completed := time.Now()
	task.Status = TaskStatusCompleted
	task.CompletedAt = &completed
	return nil
}

// PendingCount returns the number of pending tasks in the stream.
func (s *TaskScheduler) PendingCount(ctx context.Context) (int64, error) {
	if s.client == nil {
		return 0, nil
	}
	info, err := s.client.XInfoStream(ctx, s.config.StreamName).Result()
	if err != nil {
		return 0, err
	}
	return info.Length, nil
}
