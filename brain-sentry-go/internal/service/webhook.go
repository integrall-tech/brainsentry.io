package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// WebhookService manages webhook registrations and event delivery.
type WebhookService struct {
	webhookRepo *postgres.WebhookRepository
	webhooks    map[string]*domain.Webhook // in-memory cache
	byTenant    map[string][]string        // tenantID -> webhook IDs
	mu          sync.RWMutex
	client      *http.Client
	maxRetries  int
}

// NewWebhookService creates a new WebhookService.
// webhookRepo can be nil for pure in-memory mode (tests).
func NewWebhookService(webhookRepo *postgres.WebhookRepository) *WebhookService {
	return &WebhookService{
		webhookRepo: webhookRepo,
		webhooks:    make(map[string]*domain.Webhook),
		byTenant:    make(map[string][]string),
		client:      &http.Client{Timeout: 10 * time.Second},
		maxRetries:  3,
	}
}

// LoadFromDB populates the in-memory cache from the database.
func (s *WebhookService) LoadFromDB(ctx context.Context, tenantID string) {
	if s.webhookRepo == nil {
		return
	}
	webhooks, err := s.webhookRepo.FindByTenant(ctx, tenantID)
	if err != nil {
		slog.Warn("failed to load webhooks from db", "error", err)
		return
	}
	s.mu.Lock()
	for _, wh := range webhooks {
		s.webhooks[wh.ID] = wh
		s.byTenant[wh.TenantID] = append(s.byTenant[wh.TenantID], wh.ID)
	}
	s.mu.Unlock()
}

// Register registers a new webhook.
func (s *WebhookService) Register(_ context.Context, tenantID, url, secret string, events []domain.WebhookEventType) *domain.Webhook {
	webhook := &domain.Webhook{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		URL:       url,
		Secret:    secret,
		Events:    events,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Persist
	if s.webhookRepo != nil {
		if err := s.webhookRepo.Create(context.Background(), webhook); err != nil {
			slog.Warn("failed to persist webhook", "error", err)
		}
	}

	s.mu.Lock()
	s.webhooks[webhook.ID] = webhook
	s.byTenant[tenantID] = append(s.byTenant[tenantID], webhook.ID)
	s.mu.Unlock()

	return webhook
}

// Unregister removes a webhook.
func (s *WebhookService) Unregister(_ context.Context, webhookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wh, ok := s.webhooks[webhookID]
	if !ok {
		return fmt.Errorf("webhook not found: %s", webhookID)
	}

	// Persist
	if s.webhookRepo != nil {
		if err := s.webhookRepo.Delete(context.Background(), webhookID); err != nil {
			slog.Warn("failed to delete webhook from db", "error", err)
		}
	}

	delete(s.webhooks, webhookID)

	// Remove from tenant index
	ids := s.byTenant[wh.TenantID]
	for i, id := range ids {
		if id == webhookID {
			s.byTenant[wh.TenantID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}

	return nil
}

// ListWebhooks returns all webhooks for a tenant.
func (s *WebhookService) ListWebhooks(_ context.Context, tenantID string) []*domain.Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byTenant[tenantID]
	result := make([]*domain.Webhook, 0, len(ids))
	for _, id := range ids {
		if wh, ok := s.webhooks[id]; ok {
			result = append(result, wh)
		}
	}
	return result
}

// GetDeliveries returns recent deliveries for a webhook.
func (s *WebhookService) GetDeliveries(ctx context.Context, webhookID string, limit int) []domain.WebhookDelivery {
	if limit <= 0 {
		limit = 20
	}

	// Try database first
	if s.webhookRepo != nil {
		deliveries, err := s.webhookRepo.FindDeliveries(ctx, webhookID, limit)
		if err == nil {
			return deliveries
		}
		slog.Warn("failed to load deliveries from db", "error", err)
	}

	return nil
}

// Emit sends an event to all matching webhooks for a tenant.
func (s *WebhookService) Emit(tenantID string, event domain.WebhookEventType, payload any) {
	s.mu.RLock()
	ids := s.byTenant[tenantID]
	var targets []*domain.Webhook
	for _, id := range ids {
		wh := s.webhooks[id]
		if wh != nil && wh.Active && s.matchesEvent(wh, event) {
			targets = append(targets, wh)
		}
	}
	s.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"event":     event,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"tenantId":  tenantID,
		"data":      payload,
	})
	if err != nil {
		slog.Warn("failed to marshal webhook payload", "error", err)
		return
	}

	for _, wh := range targets {
		go s.deliver(wh, event, payloadBytes)
	}
}

func (s *WebhookService) matchesEvent(wh *domain.Webhook, event domain.WebhookEventType) bool {
	if len(wh.Events) == 0 {
		return true // Subscribe to all if no filter
	}
	for _, e := range wh.Events {
		if e == event {
			return true
		}
	}
	return false
}

func (s *WebhookService) deliver(wh *domain.Webhook, event domain.WebhookEventType, payload []byte) {
	start := time.Now()
	delivery := domain.WebhookDelivery{
		ID:        uuid.New().String(),
		WebhookID: wh.ID,
		Event:     event,
		Payload:   string(payload),
		Timestamp: start,
	}

	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second) // quadratic backoff
		}

		req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Event", string(event))
		req.Header.Set("X-Webhook-ID", wh.ID)
		req.Header.Set("X-Webhook-Delivery", delivery.ID)

		// Sign payload with HMAC-SHA256 if secret is set
		if wh.Secret != "" {
			sig := computeHMAC(payload, wh.Secret)
			req.Header.Set("X-Webhook-Signature", "sha256="+sig)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		delivery.StatusCode = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			delivery.Success = true
			delivery.LatencyMs = time.Since(start).Milliseconds()
			s.recordDelivery(&delivery)

			// FailCount tracks *consecutive* failures — a success resets it.
			s.mu.Lock()
			if wh.FailCount > 0 {
				wh.FailCount = 0
				wh.LastError = ""
				wh.UpdatedAt = time.Now()
			}
			s.mu.Unlock()
			return
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	// All retries failed
	delivery.Success = false
	delivery.Error = lastErr.Error()
	delivery.LatencyMs = time.Since(start).Milliseconds()
	s.recordDelivery(&delivery)

	// Update webhook fail count. Copy the struct under the lock so the
	// repository marshals a stable snapshot — concurrent deliveries mutate
	// the shared *domain.Webhook.
	s.mu.Lock()
	wh.FailCount++
	wh.LastError = lastErr.Error()
	wh.UpdatedAt = time.Now()
	// Auto-disable after 10 consecutive failures
	if wh.FailCount >= 10 {
		wh.Active = false
		slog.Warn("webhook auto-disabled after failures", "id", wh.ID, "url", wh.URL, "failCount", wh.FailCount)
	}
	snapshot := *wh
	s.mu.Unlock()

	// Persist failure state
	if s.webhookRepo != nil {
		if err := s.webhookRepo.Update(context.Background(), &snapshot); err != nil {
			slog.Warn("failed to persist webhook failure state", "id", snapshot.ID, "error", err)
		}
	}
}

func (s *WebhookService) recordDelivery(d *domain.WebhookDelivery) {
	if s.webhookRepo != nil {
		if err := s.webhookRepo.CreateDelivery(context.Background(), d); err != nil {
			slog.Warn("failed to persist webhook delivery", "error", err)
		}
	}
}

func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
