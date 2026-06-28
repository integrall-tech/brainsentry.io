package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// ConnectorType identifies the external source type.
type ConnectorType string

const (
	ConnectorGitHub      ConnectorType = "github"
	ConnectorNotion      ConnectorType = "notion"
	ConnectorGoogleDrive ConnectorType = "google_drive"
	ConnectorWebCrawler  ConnectorType = "web_crawler"
)

// ConnectorStatus represents the sync state.
type ConnectorStatus string

const (
	ConnectorStatusIdle       ConnectorStatus = "idle"
	ConnectorStatusSyncing    ConnectorStatus = "syncing"
	ConnectorStatusError      ConnectorStatus = "error"
	ConnectorStatusDisabled   ConnectorStatus = "disabled"
)

// ConnectorConfig holds configuration for an external connector.
type ConnectorConfig struct {
	Type         ConnectorType     `json:"type"`
	Name         string            `json:"name"`
	Enabled      bool              `json:"enabled"`
	Credentials  map[string]string `json:"credentials,omitempty"` // token, apiKey, etc.
	Settings     map[string]string `json:"settings,omitempty"`    // repo, workspace, folder, etc.
	SyncInterval time.Duration     `json:"syncInterval"`
	MaxDocuments int               `json:"maxDocuments"`
}

// Document represents a document fetched from an external source.
type Document struct {
	ID          string            `json:"id"`
	SourceType  ConnectorType     `json:"sourceType"`
	SourceID    string            `json:"sourceId"`    // external ID
	SourceURL   string            `json:"sourceUrl"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	ContentType string            `json:"contentType"` // text, code, markdown, html
	Metadata    map[string]string `json:"metadata,omitempty"`
	FetchedAt   time.Time         `json:"fetchedAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Checksum    string            `json:"checksum"` // for change detection
}

// DocumentChunk represents a chunk of a document ready for embedding.
type DocumentChunk struct {
	ID         string `json:"id"`
	DocumentID string `json:"documentId"`
	Content    string `json:"content"`
	Index      int    `json:"index"`
	TokenCount int    `json:"tokenCount"`
}

// SyncResult represents the outcome of a sync operation.
type SyncResult struct {
	ConnectorType ConnectorType `json:"connectorType"`
	DocumentsNew     int       `json:"documentsNew"`
	DocumentsUpdated int       `json:"documentsUpdated"`
	DocumentsSkipped int       `json:"documentsSkipped"`
	ChunksCreated    int       `json:"chunksCreated"`
	Errors           []string  `json:"errors,omitempty"`
	Duration         time.Duration `json:"duration"`
	SyncedAt         time.Time `json:"syncedAt"`
}

// Connector is the interface that all external connectors must implement.
type Connector interface {
	// Type returns the connector type.
	Type() ConnectorType
	// Name returns the connector display name.
	Name() string
	// Validate checks if the connector configuration is valid.
	Validate() error
	// FetchDocuments retrieves documents from the external source.
	FetchDocuments(ctx context.Context, since *time.Time) ([]Document, error)
	// Status returns the current connector status.
	Status() ConnectorStatus
}

// ConnectorRegistry manages registered connectors.
type ConnectorRegistry struct {
	connectors map[string]Connector
	mu         sync.RWMutex
}

// NewConnectorRegistry creates a new ConnectorRegistry.
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{
		connectors: make(map[string]Connector),
	}
}

// Register adds a connector to the registry.
func (r *ConnectorRegistry) Register(name string, connector Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[name] = connector
}

// Get retrieves a connector by name.
func (r *ConnectorRegistry) Get(name string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[name]
	return c, ok
}

// List returns all registered connectors.
func (r *ConnectorRegistry) List() map[string]Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Connector, len(r.connectors))
	for k, v := range r.connectors {
		result[k] = v
	}
	return result
}

// ConnectorService orchestrates document fetching, chunking, and memory creation.
type ConnectorService struct {
	registry    *ConnectorRegistry
	scheduler   *TaskScheduler
	memoryService *MemoryService // ingestion target for chunks (set via WithMemoryService)
	chunkSize   int // max tokens per chunk
	chunkOverlap int // overlap tokens between chunks
}

// NewConnectorService creates a new ConnectorService.
func NewConnectorService(registry *ConnectorRegistry, scheduler *TaskScheduler) *ConnectorService {
	return &ConnectorService{
		registry:     registry,
		scheduler:    scheduler,
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// WithMemoryService sets the ingestion target. Required for EmbeddingTaskHandler
// to turn document chunks into memories; without it the handler is a no-op error.
func (s *ConnectorService) WithMemoryService(m *MemoryService) *ConnectorService {
	s.memoryService = m
	return s
}

// embeddingPayload is the scheduler payload for a connector chunk awaiting
// ingestion. The tenant travels on the AsyncTask (task.TenantID), not here.
type embeddingPayload struct {
	ChunkID    string `json:"chunkId"`
	DocumentID string `json:"documentId"`
	Content    string `json:"content"`
}

// EmbeddingTaskHandler returns the TaskScheduler handler for TaskEmbedding. It
// ingests a connector document chunk as a memory, reusing the full memory
// pipeline (embedding generation, pgvector storage, graph + extraction). This is
// what makes connector sync actually land data: without it, submitted embedding
// tasks are acked and dropped. Register it for TaskEmbedding in the composition
// root, after WithMemoryService.
func (s *ConnectorService) EmbeddingTaskHandler() TaskHandler {
	return func(ctx context.Context, task *AsyncTask) error {
		if s.memoryService == nil {
			return fmt.Errorf("connector embedding handler: memory service not configured")
		}
		var p embeddingPayload
		if err := json.Unmarshal(task.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal embedding payload: %w", err)
		}
		if strings.TrimSpace(p.Content) == "" {
			return nil // empty chunk, nothing to ingest
		}
		// A memory without a tenant would leak across tenants (see CLAUDE.md), so
		// drop the task rather than create an unscoped memory.
		if task.TenantID == "" {
			slog.Warn("connector embedding task missing tenant; dropping", "chunkId", p.ChunkID)
			return nil
		}
		bgCtx := tenant.WithTenant(ctx, task.TenantID)
		_, err := s.memoryService.CreateMemory(bgCtx, dto.CreateMemoryRequest{
			Content:    truncate(p.Content, 10000),
			SourceType: "connector",
			Metadata: map[string]any{
				"source":     "connector",
				"chunkId":    p.ChunkID,
				"documentId": p.DocumentID,
			},
		})
		if err != nil {
			return fmt.Errorf("create memory from chunk %s: %w", p.ChunkID, err)
		}
		return nil
	}
}

// SyncConnector triggers a sync for a specific connector.
func (s *ConnectorService) SyncConnector(ctx context.Context, connectorName string, since *time.Time) (*SyncResult, error) {
	connector, ok := s.registry.Get(connectorName)
	if !ok {
		return nil, fmt.Errorf("connector not found: %s", connectorName)
	}

	if err := connector.Validate(); err != nil {
		return nil, fmt.Errorf("connector validation failed: %w", err)
	}

	start := time.Now()
	result := &SyncResult{
		ConnectorType: connector.Type(),
		SyncedAt:      start,
	}

	// Fetch documents
	docs, err := connector.FetchDocuments(ctx, since)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.Duration = time.Since(start)
		return result, nil
	}

	// Carry the caller's tenant onto every task so the embedding handler creates
	// tenant-scoped memories (the route is tenant-authenticated).
	tenantID := tenant.FromContext(ctx)

	// Process each document
	for _, doc := range docs {
		chunks := s.chunkDocument(doc)
		result.ChunksCreated += len(chunks)
		result.DocumentsNew++

		// Submit embedding tasks if scheduler available
		if s.scheduler != nil {
			for _, chunk := range chunks {
				if _, err := s.scheduler.Submit(ctx, TaskEmbedding, tenantID, "", PriorityNormal, embeddingPayload{
					ChunkID:    chunk.ID,
					DocumentID: chunk.DocumentID,
					Content:    chunk.Content,
				}); err != nil {
					slog.Warn("connector: failed to submit embedding task", "chunkId", chunk.ID, "error", err)
				}
			}
		}
	}

	result.Duration = time.Since(start)

	slog.Info("connector sync completed",
		"connector", connectorName,
		"type", connector.Type(),
		"documents", result.DocumentsNew,
		"chunks", result.ChunksCreated,
		"duration", result.Duration,
	)

	return result, nil
}

// SyncAll triggers sync for all enabled connectors.
func (s *ConnectorService) SyncAll(ctx context.Context, since *time.Time) map[string]*SyncResult {
	results := make(map[string]*SyncResult)

	for name, connector := range s.registry.List() {
		if connector.Status() == ConnectorStatusDisabled {
			continue
		}

		result, err := s.SyncConnector(ctx, name, since)
		if err != nil {
			results[name] = &SyncResult{
				ConnectorType: connector.Type(),
				Errors:        []string{err.Error()},
			}
		} else {
			results[name] = result
		}
	}

	return results
}

// chunkDocument splits a document into chunks for embedding.
func (s *ConnectorService) chunkDocument(doc Document) []DocumentChunk {
	content := doc.Content
	if content == "" {
		return nil
	}

	// Split by content type
	var chunks []DocumentChunk
	switch doc.ContentType {
	case "code":
		chunks = s.chunkCode(doc.ID, content)
	default:
		chunks = s.chunkText(doc.ID, content)
	}

	return chunks
}

// chunkText splits text content into overlapping chunks by token estimate.
func (s *ConnectorService) chunkText(docID, content string) []DocumentChunk {
	words := strings.Fields(content)
	if len(words) == 0 {
		return nil
	}

	// Estimate ~1.3 tokens per word
	wordsPerChunk := int(float64(s.chunkSize) / 1.3)
	overlapWords := int(float64(s.chunkOverlap) / 1.3)

	if wordsPerChunk <= 0 {
		wordsPerChunk = 100
	}

	var chunks []DocumentChunk
	idx := 0
	chunkIndex := 0

	for idx < len(words) {
		end := idx + wordsPerChunk
		if end > len(words) {
			end = len(words)
		}

		chunkContent := strings.Join(words[idx:end], " ")
		chunks = append(chunks, DocumentChunk{
			ID:         fmt.Sprintf("%s-chunk-%d", docID, chunkIndex),
			DocumentID: docID,
			Content:    chunkContent,
			Index:      chunkIndex,
			TokenCount: estimateTokens(chunkContent),
		})

		chunkIndex++
		idx = end - overlapWords
		if idx <= 0 || idx >= len(words) {
			break
		}
		if end >= len(words) {
			break
		}
	}

	return chunks
}

// chunkCode splits code by logical boundaries (functions, classes).
func (s *ConnectorService) chunkCode(docID, content string) []DocumentChunk {
	// Split by double newlines as a simple heuristic for code blocks
	blocks := strings.Split(content, "\n\n")

	var chunks []DocumentChunk
	var current strings.Builder
	chunkIndex := 0

	for _, block := range blocks {
		if current.Len()+len(block) > s.chunkSize*4 && current.Len() > 0 {
			// Flush current chunk
			chunks = append(chunks, DocumentChunk{
				ID:         fmt.Sprintf("%s-chunk-%d", docID, chunkIndex),
				DocumentID: docID,
				Content:    current.String(),
				Index:      chunkIndex,
				TokenCount: estimateTokens(current.String()),
			})
			chunkIndex++
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(block)
	}

	// Flush remaining
	if current.Len() > 0 {
		chunks = append(chunks, DocumentChunk{
			ID:         fmt.Sprintf("%s-chunk-%d", docID, chunkIndex),
			DocumentID: docID,
			Content:    current.String(),
			Index:      chunkIndex,
			TokenCount: estimateTokens(current.String()),
		})
	}

	return chunks
}

// --- GitHub Connector ---

// GitHubConnector fetches documents from GitHub repositories.
type GitHubConnector struct {
	config     ConnectorConfig
	httpClient *http.Client
	status     ConnectorStatus
}

// NewGitHubConnector creates a new GitHub connector.
func NewGitHubConnector(config ConnectorConfig) *GitHubConnector {
	return &GitHubConnector{
		config: config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		status: ConnectorStatusIdle,
	}
}

func (c *GitHubConnector) Type() ConnectorType { return ConnectorGitHub }
func (c *GitHubConnector) Name() string        { return c.config.Name }
func (c *GitHubConnector) Status() ConnectorStatus { return c.status }

func (c *GitHubConnector) Validate() error {
	if c.config.Credentials["token"] == "" {
		return fmt.Errorf("GitHub token required")
	}
	if c.config.Settings["repo"] == "" {
		return fmt.Errorf("GitHub repo required (owner/repo format)")
	}
	return nil
}

func (c *GitHubConnector) FetchDocuments(ctx context.Context, since *time.Time) ([]Document, error) {
	c.status = ConnectorStatusSyncing
	defer func() { c.status = ConnectorStatusIdle }()

	repo := c.config.Settings["repo"]
	token := c.config.Credentials["token"]

	// Fetch issues
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues?state=all&per_page=30&sort=updated", repo)
	if since != nil {
		url += "&since=" + since.Format(time.RFC3339)
	}

	body, err := c.githubGet(ctx, url, token)
	if err != nil {
		c.status = ConnectorStatusError
		return nil, fmt.Errorf("fetching issues: %w", err)
	}

	// Parse as raw text documents (simplified - in production would parse JSON)
	doc := Document{
		ID:          uuid.New().String(),
		SourceType:  ConnectorGitHub,
		SourceID:    repo,
		SourceURL:   fmt.Sprintf("https://github.com/%s", repo),
		Title:       fmt.Sprintf("GitHub Issues: %s", repo),
		Content:     body,
		ContentType: "text",
		Metadata:    map[string]string{"repo": repo, "type": "issues"},
		FetchedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return []Document{doc}, nil
}

func (c *GitHubConnector) githubGet(ctx context.Context, url, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// --- Notion Connector ---

// NotionConnector fetches pages from Notion workspaces.
type NotionConnector struct {
	config     ConnectorConfig
	httpClient *http.Client
	status     ConnectorStatus
}

// NewNotionConnector creates a new Notion connector.
func NewNotionConnector(config ConnectorConfig) *NotionConnector {
	return &NotionConnector{
		config: config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		status: ConnectorStatusIdle,
	}
}

func (c *NotionConnector) Type() ConnectorType     { return ConnectorNotion }
func (c *NotionConnector) Name() string             { return c.config.Name }
func (c *NotionConnector) Status() ConnectorStatus  { return c.status }

func (c *NotionConnector) Validate() error {
	if c.config.Credentials["token"] == "" {
		return fmt.Errorf("Notion integration token required")
	}
	return nil
}

func (c *NotionConnector) FetchDocuments(ctx context.Context, since *time.Time) ([]Document, error) {
	c.status = ConnectorStatusSyncing
	defer func() { c.status = ConnectorStatusIdle }()

	token := c.config.Credentials["token"]

	// Search for pages
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.notion.com/v1/search",
		strings.NewReader(`{"filter":{"property":"object","value":"page"},"page_size":20}`),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.status = ConnectorStatusError
		return nil, fmt.Errorf("fetching Notion pages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.status = ConnectorStatusError
		return nil, fmt.Errorf("Notion API returned %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	doc := Document{
		ID:          uuid.New().String(),
		SourceType:  ConnectorNotion,
		SourceID:    "notion-workspace",
		Title:       "Notion Pages",
		Content:     string(bodyBytes),
		ContentType: "text",
		Metadata:    map[string]string{"type": "pages"},
		FetchedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return []Document{doc}, nil
}

// --- Web Crawler Connector ---

// WebCrawlerConnector fetches content from web URLs.
type WebCrawlerConnector struct {
	config     ConnectorConfig
	httpClient *http.Client
	status     ConnectorStatus
}

// NewWebCrawlerConnector creates a new web crawler connector.
func NewWebCrawlerConnector(config ConnectorConfig) *WebCrawlerConnector {
	return &WebCrawlerConnector{
		config: config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		status: ConnectorStatusIdle,
	}
}

func (c *WebCrawlerConnector) Type() ConnectorType     { return ConnectorWebCrawler }
func (c *WebCrawlerConnector) Name() string             { return c.config.Name }
func (c *WebCrawlerConnector) Status() ConnectorStatus  { return c.status }

func (c *WebCrawlerConnector) Validate() error {
	urls := c.config.Settings["urls"]
	if urls == "" {
		return fmt.Errorf("at least one URL required in settings.urls (comma-separated)")
	}
	return nil
}

func (c *WebCrawlerConnector) FetchDocuments(ctx context.Context, since *time.Time) ([]Document, error) {
	c.status = ConnectorStatusSyncing
	defer func() { c.status = ConnectorStatusIdle }()

	urls := strings.Split(c.config.Settings["urls"], ",")
	var docs []Document

	for _, rawURL := range urls {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		content, err := c.fetchURL(ctx, rawURL)
		if err != nil {
			slog.Warn("failed to crawl URL", "url", rawURL, "error", err)
			continue
		}

		docs = append(docs, Document{
			ID:          uuid.New().String(),
			SourceType:  ConnectorWebCrawler,
			SourceID:    rawURL,
			SourceURL:   rawURL,
			Title:       fmt.Sprintf("Web: %s", rawURL),
			Content:     content,
			ContentType: "html",
			FetchedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})
	}

	return docs, nil
}

func (c *WebCrawlerConnector) fetchURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "BrainSentry-Crawler/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// --- Google Drive Connector ---

// GoogleDriveConnector fetches documents from Google Drive.
type GoogleDriveConnector struct {
	config     ConnectorConfig
	httpClient *http.Client
	status     ConnectorStatus
}

// NewGoogleDriveConnector creates a new Google Drive connector.
func NewGoogleDriveConnector(config ConnectorConfig) *GoogleDriveConnector {
	return &GoogleDriveConnector{
		config: config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		status: ConnectorStatusIdle,
	}
}

func (c *GoogleDriveConnector) Type() ConnectorType     { return ConnectorGoogleDrive }
func (c *GoogleDriveConnector) Name() string             { return c.config.Name }
func (c *GoogleDriveConnector) Status() ConnectorStatus  { return c.status }

func (c *GoogleDriveConnector) Validate() error {
	if c.config.Credentials["token"] == "" {
		return fmt.Errorf("Google Drive OAuth token required")
	}
	return nil
}

func (c *GoogleDriveConnector) FetchDocuments(ctx context.Context, since *time.Time) ([]Document, error) {
	c.status = ConnectorStatusSyncing
	defer func() { c.status = ConnectorStatusIdle }()

	token := c.config.Credentials["token"]

	url := "https://www.googleapis.com/drive/v3/files?pageSize=20&fields=files(id,name,mimeType,modifiedTime)"
	if since != nil {
		url += "&q=modifiedTime>'" + since.Format(time.RFC3339) + "'"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.status = ConnectorStatusError
		return nil, fmt.Errorf("fetching Google Drive files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.status = ConnectorStatusError
		return nil, fmt.Errorf("Google Drive API returned %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	doc := Document{
		ID:          uuid.New().String(),
		SourceType:  ConnectorGoogleDrive,
		SourceID:    "google-drive",
		Title:       "Google Drive Files",
		Content:     string(bodyBytes),
		ContentType: "text",
		Metadata:    map[string]string{"type": "file_list"},
		FetchedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return []Document{doc}, nil
}
