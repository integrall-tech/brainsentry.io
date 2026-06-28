package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// SessionConfig holds session lifecycle configuration.
type SessionConfig struct {
	DefaultTTL      time.Duration // Default session TTL (default: 2h)
	MaxIdleTime     time.Duration // Max idle before auto-expire (default: 30m)
	CleanupInterval time.Duration // How often to run cleanup (default: 5m)
}

// DefaultSessionConfig returns defaults.
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		DefaultTTL:      2 * time.Hour,
		MaxIdleTime:     30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
}

// SessionService manages session lifecycle with PostgreSQL backing and in-memory cache.
type SessionService struct {
	sessionRepo *postgres.SessionRepository
	cache       map[string]*domain.Session // in-memory cache
	mu          sync.RWMutex
	config      SessionConfig
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewSessionService creates a new SessionService.
// sessionRepo can be nil for pure in-memory mode (tests).
func NewSessionService(config SessionConfig, sessionRepo *postgres.SessionRepository) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
		cache:       make(map[string]*domain.Session),
		config:      config,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the background cleanup goroutine.
func (s *SessionService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
}

// Stop gracefully stops the cleanup goroutine.
func (s *SessionService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// CreateSession starts a new session.
func (s *SessionService) CreateSession(ctx context.Context, userID string) *domain.Session {
	now := time.Now()
	session := &domain.Session{
		ID:             uuid.New().String(),
		UserID:         userID,
		TenantID:       tenant.FromContext(ctx),
		Status:         domain.SessionActive,
		StartedAt:      now,
		LastActivityAt: now,
		ExpiresAt:      now.Add(s.config.DefaultTTL),
	}

	// Persist to database
	if s.sessionRepo != nil {
		if err := s.sessionRepo.Create(ctx, session); err != nil {
			slog.Warn("failed to persist session", "error", err)
		}
	}

	// Cache
	s.mu.Lock()
	s.cache[session.ID] = session
	s.mu.Unlock()

	return session
}

// GetSession retrieves a session by ID.
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	// Check cache first
	s.mu.RLock()
	session, ok := s.cache[sessionID]
	s.mu.RUnlock()

	if !ok && s.sessionRepo != nil {
		// Try database
		var err error
		session, err = s.sessionRepo.FindByID(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		// Populate cache
		s.mu.Lock()
		s.cache[sessionID] = session
		s.mu.Unlock()
	} else if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Auto-expire if past expiry
	if session.IsExpired() && session.Status == domain.SessionActive {
		s.expireSession(ctx, session)
	}

	return session, nil
}

// TouchSession updates the last activity timestamp and extends expiry if idle.
func (s *SessionService) TouchSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	session, ok := s.cache[sessionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != domain.SessionActive {
		s.mu.Unlock()
		return fmt.Errorf("session is not active: %s", session.Status)
	}

	now := time.Now()
	session.LastActivityAt = now

	// Extend expiry if close to expiring
	remaining := time.Until(session.ExpiresAt)
	if remaining < s.config.MaxIdleTime {
		session.ExpiresAt = now.Add(s.config.MaxIdleTime)
	}
	s.mu.Unlock()

	// Persist
	if s.sessionRepo != nil {
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			slog.Warn("failed to persist session touch", "error", err)
		}
	}

	return nil
}

// EndSession completes a session.
func (s *SessionService) EndSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	session, ok := s.cache[sessionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}

	now := time.Now()
	session.Status = domain.SessionCompleted
	session.EndedAt = &now
	s.mu.Unlock()

	// Persist
	if s.sessionRepo != nil {
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			slog.Warn("failed to persist session end", "error", err)
		}
	}

	return nil
}

// ListActiveSessions returns all active sessions for a tenant.
func (s *SessionService) ListActiveSessions(ctx context.Context) []*domain.Session {
	// Try database first for completeness
	if s.sessionRepo != nil {
		sessions, err := s.sessionRepo.FindActiveByTenant(ctx)
		if err == nil {
			return sessions
		}
		slog.Warn("failed to list sessions from db, using cache", "error", err)
	}

	tenantID := tenant.FromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Session
	for _, session := range s.cache {
		if session.TenantID == tenantID && session.IsActive() {
			result = append(result, session)
		}
	}
	return result
}

// IncrementMemoryCount increments the memory counter for a session.
func (s *SessionService) IncrementMemoryCount(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.cache[sessionID]; ok {
		session.MemoryCount++
	}
}

// IncrementInterceptionCount increments the interception counter.
func (s *SessionService) IncrementInterceptionCount(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.cache[sessionID]; ok {
		session.InterceptionCount++
	}
}

func (s *SessionService) expireSession(ctx context.Context, session *domain.Session) {
	s.mu.Lock()
	if session.Status == domain.SessionActive {
		now := time.Now()
		session.Status = domain.SessionExpired
		session.EndedAt = &now
	}
	s.mu.Unlock()

	if s.sessionRepo != nil {
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			slog.Warn("failed to persist session update", "sessionId", session.ID, "error", err)
		}
	}
}

func (s *SessionService) cleanup() {
	// Database cleanup
	if s.sessionRepo != nil {
		expired, err := s.sessionRepo.ExpireOld(context.Background(), s.config.MaxIdleTime)
		if err != nil {
			slog.Warn("session db cleanup failed", "error", err)
		} else if expired > 0 {
			slog.Info("session db cleanup", "expired", expired)
		}

		deleted, err := s.sessionRepo.DeleteOldCompleted(context.Background(), 24*time.Hour)
		if err != nil {
			slog.Warn("session db cleanup delete failed", "error", err)
		} else if deleted > 0 {
			slog.Info("session db cleanup", "deleted", deleted)
		}
	}

	// In-memory cache cleanup
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expired := 0

	for _, session := range s.cache {
		if session.Status != domain.SessionActive {
			continue
		}
		if now.After(session.ExpiresAt) || now.Sub(session.LastActivityAt) > s.config.MaxIdleTime {
			endTime := now
			session.Status = domain.SessionExpired
			session.EndedAt = &endTime
			expired++
		}
	}

	if expired > 0 {
		slog.Info("session cache cleanup", "expired", expired)
	}

	// Remove old completed/expired from cache
	for id, session := range s.cache {
		if session.EndedAt != nil && now.Sub(*session.EndedAt) > 24*time.Hour {
			delete(s.cache, id)
		}
	}
}
