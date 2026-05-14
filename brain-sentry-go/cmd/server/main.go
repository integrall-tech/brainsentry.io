package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/integraltech/brainsentry/internal/cache"
	"github.com/integraltech/brainsentry/internal/config"
	"github.com/integraltech/brainsentry/internal/diagnostics"
	"github.com/integraltech/brainsentry/internal/eval"
	"github.com/integraltech/brainsentry/internal/handler"
	modelsrouting "github.com/integraltech/brainsentry/internal/models"
	"github.com/integraltech/brainsentry/internal/mcp"
	"github.com/integraltech/brainsentry/internal/middleware"
	"github.com/integraltech/brainsentry/internal/rebuild"
	graphrepo "github.com/integraltech/brainsentry/internal/repository/graph"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"github.com/integraltech/brainsentry/internal/service"
	"github.com/integraltech/brainsentry/pkg/lazy"
	"github.com/integraltech/brainsentry/pkg/trust"
)

func main() {
	// Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Operator-mode flags. --rebuild + --confirm-destructive run the
	// rebuild executor in-process after services are wired, then exit.
	// Trust elevation lives here (not in the HTTP path) — process is
	// already on the operator's host.
	rebuildTargets := flag.String("rebuild", "", "comma-separated rebuild targets (e.g. graph,embeddings,communities,compress); when set, run them and exit")
	rebuildConfirm := flag.Bool("confirm-destructive", false, "required to actually run --rebuild; without it, rebuild dry-runs")
	flag.Parse()

	// Config
	cfgPath := "config.yaml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		cfgPath = p
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Database
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.Database.DSN(), cfg.Database.MaxConnections, cfg.Database.MinConnections)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	// FalkorDB (optional - non-fatal if unavailable)
	var graphClient *graphrepo.Client
	var memoryGraphRepo *graphrepo.MemoryGraphRepository
	var entityGraphRepo *graphrepo.EntityGraphRepository

	var graphRAGRepo *graphrepo.GraphRAGRepository

	graphClient, err = graphrepo.NewClient(cfg.FalkorDB.Addr(), cfg.FalkorDB.Password, cfg.FalkorDB.GraphName)
	if err != nil {
		logger.Warn("FalkorDB not available, graph features disabled", "error", err)
	} else {
		defer graphClient.Close()
		memoryGraphRepo = graphrepo.NewMemoryGraphRepository(graphClient)
		entityGraphRepo = graphrepo.NewEntityGraphRepository(graphClient)
		graphRAGRepo = graphrepo.NewGraphRAGRepository(graphClient)

		// Ensure vector index exists — lazily so boot is fast. The index is
		// created on first semantic search instead of during startup.
		vectorIndexReady := lazy.New(func() (bool, error) {
			if err := graphRAGRepo.EnsureVectorIndex(context.Background(), cfg.Embedding.Dimensions); err != nil {
				return false, err
			}
			return true, nil
		})
		graphRAGRepo.SetIndexInitializer(func(ctx context.Context) error {
			_, err := vectorIndexReady.Get()
			return err
		})

		logger.Info("connected to FalkorDB", "graph", cfg.FalkorDB.GraphName)
	}

	// Redis (optional - non-fatal if unavailable)
	redisCache := cache.NewRedisCache(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
	if redisCache != nil {
		defer redisCache.Close()
	}
	_ = redisCache // used by rate limiter and future caching

	// Repositories
	userRepo := postgres.NewUserRepository(pool)
	tenantRepo := postgres.NewTenantRepository(pool)
	memoryRepo := postgres.NewMemoryRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	versionRepo := postgres.NewVersionRepository(pool)
	relRepo := postgres.NewRelationshipRepository(pool)
	noteRepo := postgres.NewNoteRepository(pool)
	decisionRepo := postgres.NewDecisionRepository(pool)
	policyRepo := postgres.NewPolicyRepository(pool)
	eventRepo := postgres.NewEventRepository(pool)

	// Services
	jwtService := service.NewJWTService(cfg.Security.JWTSecret, cfg.Security.JWTExpiration)
	authService := service.NewAuthService(userRepo, jwtService)
	auditService := service.NewAuditService(auditRepo)
	embeddingService := service.NewEmbeddingService(cfg.Embedding.Dimensions, cfg.AI.APIKey, cfg.AI.BaseURL, cfg.Embedding.Model)

	var openRouterService *service.OpenRouterService
	if cfg.AI.APIKey != "" {
		openRouterService = service.NewOpenRouterService(
			cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model,
			cfg.AI.Temperature, cfg.AI.MaxTokens, cfg.AI.Timeout, cfg.AI.MaxRetries,
		)
		logger.Info("OpenRouter service initialized", "model", cfg.AI.Model)
	} else {
		logger.Warn("OpenRouter API key not set, LLM features disabled")
	}

	memoryService := service.NewMemoryService(
		memoryRepo, versionRepo, memoryGraphRepo, auditService,
		openRouterService, embeddingService, cfg.Memory.AutoImportance,
	)

	interceptionService := service.NewInterceptionService(
		memoryRepo, memoryGraphRepo, graphRAGRepo, openRouterService, embeddingService,
		auditService, noteRepo,
		cfg.Interception.QuickCheckEnabled, cfg.Interception.DeepAnalysisEnabled,
		cfg.Interception.RelevanceThreshold,
	)

	var entityGraphService *service.EntityGraphService
	if entityGraphRepo != nil {
		entityGraphService = service.NewEntityGraphService(entityGraphRepo, openRouterService, auditService)
	}

	relationshipService := service.NewRelationshipService(relRepo, memoryRepo, openRouterService, auditService)

	// Phase 4: NoteTaking, Compression, MCP
	summaryRepo := postgres.NewContextSummaryRepository(pool)
	observationRepo := postgres.NewObservationRepository(pool)
	noteTakingService := service.NewNoteTakingService(noteRepo, auditRepo, openRouterService, auditService, observationRepo)
	compressionService := service.NewCompressionService(summaryRepo, openRouterService)

	// Learning Service (background auto-promotion/demotion)
	learningService := service.NewLearningService(
		memoryRepo, tenantRepo, auditService, service.DefaultLearningConfig(),
	)
	learningService.Start([]string{cfg.Tenant.DefaultID})

	// Consolidation Service
	consolidationService := service.NewConsolidationService(
		memoryRepo, openRouterService, embeddingService, auditService,
	)

	// Correction Service
	correctionService := service.NewCorrectionService(memoryRepo, versionRepo, auditService)

	// Profile Service
	var profileService *service.ProfileService
	if openRouterService != nil {
		profileService = service.NewProfileService(openRouterService, memoryRepo)
	}

	// NL Cypher Service (requires FalkorDB + LLM)
	var nlCypherService *service.NLCypherService
	if openRouterService != nil && graphClient != nil {
		nlCypherService = service.NewNLCypherService(openRouterService, graphClient)
	}

	// Reflection Service
	var reflectionService *service.ReflectionService
	if openRouterService != nil {
		reflectionService = service.NewReflectionService(openRouterService, memoryRepo, memoryService)
	}

	// Reconciliation Service
	var reconciliationService *service.ReconciliationService
	if openRouterService != nil {
		reconciliationService = service.NewReconciliationService(openRouterService, memoryRepo, memoryService)
	}

	// Retrieval Planner Service
	var retrievalPlannerService *service.RetrievalPlannerService
	if openRouterService != nil {
		retrievalPlannerService = service.NewRetrievalPlannerService(openRouterService, memoryRepo, memoryGraphRepo, embeddingService)
	}

	// Louvain Community Detection (requires FalkorDB)
	var louvainService *service.LouvainService
	if graphClient != nil {
		louvainService = service.NewLouvainService(graphClient)
	}

	// Spreading Activation (requires FalkorDB)
	var spreadingActivationService *service.SpreadingActivationService
	if memoryGraphRepo != nil && graphClient != nil {
		spreadingActivationService = service.NewSpreadingActivationService(memoryGraphRepo, graphClient)
	}

	// Task Scheduler (requires Redis)
	var taskScheduler *service.TaskScheduler
	if redisCache != nil {
		taskScheduler = service.NewTaskScheduler(redisCache.Client(), service.DefaultTaskSchedulerConfig())
		go func() {
			if err := taskScheduler.Start(ctx); err != nil {
				logger.Error("task scheduler start error", "error", err)
			}
		}()
	}

	// Connector Service
	connectorRegistry := service.NewConnectorRegistry()
	var connectorService *service.ConnectorService
	if taskScheduler != nil {
		connectorService = service.NewConnectorService(connectorRegistry, taskScheduler)
	}

	// Benchmark Service
	benchmarkService := service.NewBenchmarkService()

	// Circuit Breaker Registry & LLM Observer
	cbRegistry := service.NewCircuitBreakerRegistry()
	llmObserver := service.NewMetricsObserver()

	// PII Service
	piiService := service.NewPIIService()

	// ---- P1-P3 New Services ----

	// Fallback Chain LLM Provider. Order matters: the first provider with
	// credentials becomes primary, subsequent ones backstop it when the
	// primary's circuit opens. Anthropic+Gemini natives skip the OpenRouter
	// hop (lower latency, prompt caching), so when configured they go first.
	var llmProvider service.LLMProvider
	chain := make([]service.LLMProvider, 0, 3)
	if cfg.Anthropic.APIKey != "" {
		ac := service.DefaultAnthropicConfig(cfg.Anthropic.APIKey)
		if cfg.Anthropic.BaseURL != "" {
			ac.BaseURL = cfg.Anthropic.BaseURL
		}
		if cfg.Anthropic.Model != "" {
			ac.Model = cfg.Anthropic.Model
		}
		if cfg.Anthropic.MaxTokens > 0 {
			ac.MaxTokens = cfg.Anthropic.MaxTokens
		}
		if cfg.Anthropic.Temperature != 0 {
			ac.Temperature = cfg.Anthropic.Temperature
		}
		if cfg.Anthropic.Timeout > 0 {
			ac.Timeout = cfg.Anthropic.Timeout
		}
		chain = append(chain, service.NewAnthropicProvider(ac))
		logger.Info("Anthropic native provider added to chain", "model", ac.Model)
	}
	if cfg.Gemini.APIKey != "" {
		gc := service.DefaultGeminiConfig(cfg.Gemini.APIKey)
		if cfg.Gemini.BaseURL != "" {
			gc.BaseURL = cfg.Gemini.BaseURL
		}
		if cfg.Gemini.Model != "" {
			gc.Model = cfg.Gemini.Model
		}
		if cfg.Gemini.MaxTokens > 0 {
			gc.MaxTokens = cfg.Gemini.MaxTokens
		}
		if cfg.Gemini.Temperature != 0 {
			gc.Temperature = cfg.Gemini.Temperature
		}
		if cfg.Gemini.Timeout > 0 {
			gc.Timeout = cfg.Gemini.Timeout
		}
		chain = append(chain, service.NewGeminiProvider(gc))
		logger.Info("Gemini native provider added to chain", "model", gc.Model)
	}
	if openRouterService != nil {
		chain = append(chain, service.NewOpenRouterProvider(openRouterService))
	}
	if len(chain) > 0 {
		llmProvider = service.NewFallbackChainProvider(chain...)
		logger.Info("LLM fallback chain initialized", "providers", len(chain))
	}

	// Memory Compression Pipeline
	memoryCompressionService := service.NewMemoryCompressionService(llmProvider)

	// Query Expansion
	queryExpansionService := service.NewQueryExpansionService(llmProvider)

	// Self-Correcting LLM
	var selfCorrectingLLM *service.SelfCorrectingLLM
	if llmProvider != nil {
		selfCorrectingLLM = service.NewSelfCorrectingLLM(llmProvider, 2)
	}

	// Auto-Forget
	autoForgetService := service.NewAutoForgetService(memoryRepo, auditService, service.DefaultAutoForgetConfig())

	// Cascading Staleness
	cascadingStalenessService := service.NewCascadingStalenessService(memoryRepo, relRepo, auditService)

	// Semantic Memory (consolidation tiers)
	semanticMemoryService := service.NewSemanticMemoryService(memoryRepo, llmProvider, auditService)

	// Privacy Stripping
	privacyStrippingService := service.NewPrivacyStrippingService()

	// Sliding Window Enrichment
	slidingWindowService := service.NewSlidingWindowEnrichment(llmProvider)

	// Actions & Leases
	actionService := service.NewActionService()

	// Mesh Sync
	meshSyncService := service.NewMeshSyncService(cfg.Tenant.DefaultID, service.DefaultMeshSyncConfig())

	// Log new services status
	logger.Info("new services initialized",
		"compression", memoryCompressionService != nil,
		"queryExpansion", queryExpansionService != nil,
		"selfCorrecting", selfCorrectingLLM != nil,
		"autoForget", autoForgetService != nil,
		"cascadingStaleness", cascadingStalenessService != nil,
		"semanticMemory", semanticMemoryService != nil,
		"privacyStripping", privacyStrippingService != nil,
		"slidingWindow", slidingWindowService != nil,
		"actions", actionService != nil,
		"meshSync", meshSyncService != nil,
	)

	// Wire internal pipeline enhancers into MemoryService.
	// Each service is optional — MemoryService falls back to previous behavior if nil.
	memoryService.
		WithCompressor(memoryCompressionService).
		WithQueryExpander(queryExpansionService).
		WithPrivacyStripper(privacyStrippingService).
		WithCascadingStaleness(cascadingStalenessService)

	// Suppress remaining unused warnings for services that are not yet wired
	// (sliding window and self-correcting LLM are available for future integration).
	_ = selfCorrectingLLM
	_ = slidingWindowService

	// ---- P1 Cognee: Triplet Extraction, Query Router, Cascade Extraction, Feedback Learning ----

	// Query Router (rule-based, LLM-free)
	queryRouterService := service.NewQueryRouterService(service.DefaultQueryRouterConfig())

	// Feedback Learning Service
	feedbackLearningService := service.NewFeedbackLearningService(service.DefaultFeedbackLearningConfig())

	// Triplet Extraction (requires LLM)
	var tripletExtractionService *service.TripletExtractionService
	if llmProvider != nil {
		tripletExtractionService = service.NewTripletExtractionService(llmProvider)
	}

	// Cascade Entity Extraction (requires LLM)
	var cascadeExtractionService *service.CascadeEntityExtractionService
	if llmProvider != nil {
		cascadeExtractionService = service.NewCascadeEntityExtractionService(llmProvider)
	}

	logger.Info("P1-Cognee services initialized",
		"queryRouter", queryRouterService != nil,
		"feedbackLearning", feedbackLearningService != nil,
		"tripletExtraction", tripletExtractionService != nil,
		"cascadeExtraction", cascadeExtractionService != nil,
	)

	// Wire P1-Cognee enhancers into MemoryService pipeline
	memoryService.
		WithTripletExtractor(tripletExtractionService).
		WithFeedbackLearning(feedbackLearningService)

	// Wire coreference resolution into cascade extraction (defined below next to
	// Semantica services, so we attach it in a deferred pass). Handled further
	// in the boot sequence after coreferenceService is constructed.

	// ---- P2 Cognee: AgentTrace, NodeSet, Semantic API, Middleware ----

	// AgentTrace (procedural memory — in-memory for now)
	agentTraceService := service.NewAgentTraceService(service.DefaultAgentTraceConfig())

	// NodeSet (multi-set grouping via metadata JSON)
	nodeSetService := service.NewNodeSetService(memoryRepo)

	// Semantic API (high-level remember/recall/improve/forget)
	semanticAPIService := service.NewSemanticAPIService(
		memoryService,
		autoForgetService,
		feedbackLearningService,
		nodeSetService,
		agentTraceService,
		queryRouterService,
		memoryRepo,
	)

	logger.Info("P2-Cognee services initialized",
		"agentTrace", agentTraceService != nil,
		"nodeSet", nodeSetService != nil,
		"semanticAPI", semanticAPIService != nil,
	)

	// ---- P3 Cognee: Ontology, Session Cache, Native LLM Providers, Graph Backend ----

	// Ontology Service (optional — loads from ONTOLOGY_PATH env var if set)
	ontologyService := service.NewOntologyService()
	if ontPath := os.Getenv("ONTOLOGY_PATH"); ontPath != "" {
		if err := ontologyService.LoadFromFile(ontPath); err != nil {
			logger.Warn("failed to load ontology", "path", ontPath, "error", err)
		} else {
			logger.Info("ontology loaded", "path", ontPath,
				"types", len(ontologyService.AllowedTypes()),
				"relationships", len(ontologyService.AllowedRelationships()))
		}
	}

	// Session Memory Cache (Redis if available, in-memory fallback)
	var sessionBackend service.SessionCacheBackend
	if redisCache != nil {
		sessionBackend = service.NewRedisSessionBackend(redisCache.Client(), "session_cache:", 100)
		logger.Info("session cache using Redis backend")
	} else {
		sessionBackend = service.NewInMemorySessionBackend(100)
		logger.Info("session cache using in-memory backend (Redis unavailable)")
	}
	sessionCacheService := service.NewSessionMemoryCache(sessionBackend, service.DefaultSessionCacheConfig(), memoryService)

	// Native LLM providers — configure via env vars
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		_ = service.NewAnthropicProvider(service.DefaultAnthropicConfig(anthropicKey))
		logger.Info("Anthropic native provider available")
	}
	if geminiKey := os.Getenv("GEMINI_API_KEY"); geminiKey != "" {
		_ = service.NewGeminiProvider(service.DefaultGeminiConfig(geminiKey))
		logger.Info("Gemini native provider available")
	}

	logger.Info("P3-Cognee services initialized",
		"ontology", ontologyService.IsEnabled(),
		"sessionCache", sessionCacheService != nil,
	)

	// ---- Semantica-inspired services: Decisions, Policies, Events, Reasoning, Provenance ----

	policyEngine := service.NewPolicyEngine(policyRepo, auditService)

	decisionService := service.NewDecisionService(decisionRepo, embeddingService, auditService).
		WithPolicyEngine(policyEngine)

	eventService := service.NewEventService(eventRepo, embeddingService, llmProvider, auditService)

	var abductiveReasoner *service.AbductiveReasoner
	if llmProvider != nil {
		abductiveReasoner = service.NewAbductiveReasoner(llmProvider, decisionRepo, memoryRepo, decisionService)
	}

	provenanceExporter := service.NewProvenanceExporter(auditRepo, decisionRepo, os.Getenv("PROV_BASE_URI"))

	coreferenceService := service.NewCoreferenceService(llmProvider)

	// Attach coreference to cascade extraction pipeline.
	if cascadeExtractionService != nil {
		cascadeExtractionService.WithCoreference(coreferenceService)
	}

	// Fire event extraction asynchronously after each memory create.
	memoryService.WithEventExtractor(eventService)

	logger.Info("Semantica-inspired services initialized",
		"decisions", decisionService != nil,
		"policies", policyEngine != nil,
		"events", eventService != nil,
		"abductiveReasoner", abductiveReasoner != nil,
		"provenanceExporter", provenanceExporter != nil,
	)

	// Batch Search (multi-query parallel ranking)
	batchSearchService := service.NewBatchSearchService(memoryService)

	// Session Service
	sessionRepo := postgres.NewSessionRepository(pool)
	sessionService := service.NewSessionService(service.DefaultSessionConfig(), sessionRepo)
	sessionService.Start()

	// Cross-Session Service
	var crossSessionService *service.CrossSessionService
	if openRouterService != nil {
		crossSessionService = service.NewCrossSessionService(memoryRepo, openRouterService, sessionService)
	}

	// Batch Service
	batchService := service.NewBatchService(memoryRepo, embeddingService, auditService)

	// Conflict Detection Service
	conflictService := service.NewConflictService(memoryRepo, openRouterService, embeddingService)

	// Webhook Service
	webhookRepo := postgres.NewWebhookRepository(pool)
	webhookService := service.NewWebhookService(webhookRepo)

	// SSO Service
	ssoService := service.NewSSOService(jwtService)

	// MCP Server
	mcpServer := mcp.NewServer(memoryService, noteTakingService, compressionService, interceptionService)

	// Handlers
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userRepo, authService, cfg.Security.BcryptCost)
	tenantHandler := handler.NewTenantHandler(tenantRepo)
	memoryHandler := handler.NewMemoryHandler(memoryService, relationshipService)
	auditHandler := handler.NewAuditHandler(auditService)
	statsHandler := handler.NewStatsHandler(memoryRepo, auditRepo)
	interceptionHandler := handler.NewInterceptionHandler(interceptionService)
	relationshipHandler := handler.NewRelationshipHandler(relationshipService, memoryService)

	correctionHandler := handler.NewCorrectionHandler(correctionService)
	sessionHandler := handler.NewSessionHandler(sessionService)
	batchHandler := handler.NewBatchHandler(batchService)
	conflictHandler := handler.NewConflictHandler(conflictService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	ssoHandler := handler.NewSSOHandler(ssoService)
	noteTakingHandler := handler.NewNoteTakingHandler(noteTakingService)
	compressionHandler := handler.NewCompressionHandler(compressionService)
	mcpHandler := handler.NewMCPHandler(mcpServer)

	var entityGraphHandler *handler.EntityGraphHandler
	if entityGraphService != nil {
		entityGraphHandler = handler.NewEntityGraphHandler(entityGraphService, memoryService)
	}

	// New handlers for previously unexposed services
	var profileHandler *handler.ProfileHandler
	if profileService != nil {
		profileHandler = handler.NewProfileHandler(profileService)
	}

	var nlQueryHandler *handler.NLQueryHandler
	if nlCypherService != nil {
		nlQueryHandler = handler.NewNLQueryHandler(nlCypherService)
	}

	var reflectionHandler *handler.ReflectionHandler
	if reflectionService != nil {
		reflectionHandler = handler.NewReflectionHandler(reflectionService)
	}

	var reconciliationHandler *handler.ReconciliationHandler
	if reconciliationService != nil {
		reconciliationHandler = handler.NewReconciliationHandler(reconciliationService)
	}

	var retrievalHandler *handler.RetrievalHandler
	if retrievalPlannerService != nil {
		retrievalHandler = handler.NewRetrievalHandler(retrievalPlannerService)
	}

	var communitiesHandler *handler.CommunitiesHandler
	if louvainService != nil {
		communitiesHandler = handler.NewCommunitiesHandler(louvainService)
	}

	var graphViewHandler *handler.GraphViewHandler
	if memoryRepo != nil {
		graphViewHandler = handler.NewGraphViewHandler(memoryRepo, graphClient, graphRAGRepo, louvainService)
	}

	// Diagnostics ("doctor") — TCP probes for every external dependency the
	// running server can reach, plus a Postgres ping closure so the operator
	// learns immediately if connectivity has degraded.
	diagCheckers := []diagnostics.Checker{
		&diagnostics.TCPChecker{
			CheckName: "postgres",
			Sev:       diagnostics.SeverityCritical,
			Host:      cfg.Database.Host,
			Port:      cfg.Database.Port,
			Hint:      "verify docker compose ps and credentials",
		},
		&diagnostics.TCPChecker{
			CheckName: "falkordb",
			Sev:       diagnostics.SeverityWarning,
			Host:      cfg.FalkorDB.Host,
			Port:      cfg.FalkorDB.Port,
			Hint:      "graph features disabled until reachable",
		},
		&diagnostics.TCPChecker{
			CheckName: "redis",
			Sev:       diagnostics.SeverityWarning,
			Host:      cfg.Redis.Host,
			Port:      cfg.Redis.Port,
			Hint:      "rate-limit + caching degraded without redis",
		},
		&diagnostics.FuncChecker{
			CheckName: "postgres-ping",
			Fn: func(ctx context.Context) diagnostics.CheckResult {
				if err := pool.Ping(ctx); err != nil {
					return diagnostics.CheckResult{
						Status: diagnostics.StatusFail, Severity: diagnostics.SeverityCritical,
						Message: "ping failed", Detail: err.Error(),
					}
				}
				return diagnostics.CheckResult{
					Status: diagnostics.StatusOK, Severity: diagnostics.SeverityCritical,
					Message: "round-trip ok",
				}
			},
		},
	}
	if cfg.AI.APIKey != "" && cfg.AI.BaseURL != "" {
		diagCheckers = append(diagCheckers, &diagnostics.HTTPChecker{
			CheckName: "openrouter",
			Sev:       diagnostics.SeverityWarning,
			URL:       cfg.AI.BaseURL,
			Method:    "GET",
			Hint:      "verify OPENROUTER_API_KEY and account funding",
		})
	}
	doctor := diagnostics.New(diagCheckers, 4*time.Second)
	diagnosticsHandler := handler.NewDiagnosticsHandler(doctor)

	adminTrustHandler := handler.NewAdminTrustHandler(redisCache)

	// Eval candidates store — opt-in capture via BRAINSENTRY_EVAL_CAPTURE=1.
	// Ring-buffer at 5000 keeps memory bounded if the operator forgets to
	// drain. Resets after every successful export.
	evalStore := eval.NewStore(5000)
	evalHandler := handler.NewEvalHandler(evalStore)
	memoryHandler.WithEvalCapture(evalStore)

	// Tier-based model routing — operators set per-tier overrides under
	// `models:` in config.yaml; resolution falls back to AI.Model and the
	// built-in TierDefaults so existing configs keep working.
	modelsCfg := modelsrouting.FromYAML(cfg.Models.Default, cfg.Models.Tier, cfg.AI.Model)
	var modelsProber modelsrouting.Prober
	if cfg.AI.APIKey != "" && cfg.AI.BaseURL != "" {
		modelsProber = &modelsrouting.HTTPProber{
			BaseURL: cfg.AI.BaseURL,
			APIKey:  cfg.AI.APIKey,
			Client:  &http.Client{Timeout: 10 * time.Second},
		}
	}
	modelsHandler := handler.NewModelsHandler(modelsCfg, modelsProber)

	// Rebuild executor: register concrete rebuilders for every derived
	// store. Each is a thin closure over the corresponding repo/service
	// already wired above. Targets land in alphabetical order in the
	// report — see internal/rebuild/service.go for the contract.
	rebuildSvc := rebuild.New()
	if memoryGraphRepo != nil && memoryRepo != nil {
		_ = rebuildSvc.Register("graph", rebuild.GraphRebuilder(memoryRepo, memoryGraphRepo))
	}
	_ = rebuildSvc.Register("embeddings", rebuild.EmbeddingsRebuilder(memoryRepo))
	_ = rebuildSvc.Register("compress", rebuild.CompressRebuilder(memoryRepo))
	if louvainService != nil {
		commAdapter := rebuild.NewCommunityAdapter(tenantRepo, func(ctx context.Context, tenantID string) (int, error) {
			res, err := louvainService.DetectCommunities(ctx, tenantID)
			if err != nil || res == nil {
				return 0, err
			}
			return len(res.Communities), nil
		})
		_ = rebuildSvc.Register("communities", rebuild.CommunitiesRebuilder(commAdapter))
	}

	// --rebuild=<targets> mode: run executor in-process, print report,
	// exit. Skips HTTP server entirely. Trust elevation lives here, on
	// the operator's host. --confirm-destructive is required.
	if *rebuildTargets != "" {
		runRebuildAndExit(logger, rebuildSvc, *rebuildTargets, *rebuildConfirm)
	}

	var activationHandler *handler.ActivationHandler
	if spreadingActivationService != nil {
		activationHandler = handler.NewActivationHandler(spreadingActivationService)
	}

	var crossSessionHandler *handler.CrossSessionHandler
	if crossSessionService != nil {
		crossSessionHandler = handler.NewCrossSessionHandler(crossSessionService)
	}

	var connectorsHandler *handler.ConnectorsHandler
	if connectorService != nil {
		connectorsHandler = handler.NewConnectorsHandler(connectorService, connectorRegistry)
	}

	var tasksHandler *handler.TasksHandler
	if taskScheduler != nil {
		tasksHandler = handler.NewTasksHandler(taskScheduler)
	}

	consolidationHandler := handler.NewConsolidationHandler(consolidationService)
	benchmarkHandler := handler.NewBenchmarkHandler(benchmarkService, memoryService)
	adminHandler := handler.NewAdminHandler(cbRegistry, llmObserver, piiService)

	// New P1-P3 handlers
	autoForgetHandler := handler.NewAutoForgetHandler(autoForgetService)
	semanticMemoryHandler := handler.NewSemanticMemoryHandler(semanticMemoryService)
	actionsHandler := handler.NewActionsHandler(actionService)
	meshHandler := handler.NewMeshHandler(meshSyncService)

	// P1-Cognee handlers
	queryRouterHandler := handler.NewQueryRouterHandler(queryRouterService)
	tripletHandler := handler.NewTripletHandler(tripletExtractionService)
	cascadeExtractionHandler := handler.NewCascadeExtractionHandler(cascadeExtractionService)
	feedbackLearningHandler := handler.NewFeedbackLearningHandler(feedbackLearningService, memoryRepo)

	// P2-Cognee handlers
	agentTraceHandler := handler.NewAgentTraceHandler(agentTraceService)
	nodeSetHandler := handler.NewNodeSetHandler(nodeSetService, memoryRepo)
	semanticAPIHandler := handler.NewSemanticAPIHandler(semanticAPIService)

	// P3-Cognee handlers
	ontologyHandler := handler.NewOntologyHandler(ontologyService)
	sessionCacheHandler := handler.NewSessionCacheHandler(sessionCacheService)
	batchSearchHandler := handler.NewBatchSearchHandler(batchSearchService)

	// Semantica-inspired handlers
	decisionHandler := handler.NewDecisionHandler(decisionService)
	policyHandler := handler.NewPolicyHandler(policyEngine, decisionService)
	eventHandler := handler.NewEventHandler(eventService)
	reasoningHandler := handler.NewReasoningHandler(abductiveReasoner)
	provenanceHandler := handler.NewProvenanceHandler(provenanceExporter)
	biTemporalHandler := handler.NewBiTemporalHandler(memoryRepo)
	coreferenceHandler := handler.NewCoreferenceHandler(coreferenceService)

	// Router
	r := chi.NewRouter()

	// Rate limiter
	rateLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		RequestsPerMinute: 120,
		BurstSize:         60,
	})

	// Global middleware
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Metrics())
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.CORS(cfg.Security.CORS.AllowedOrigins, cfg.Security.CORS.AllowedMethods))
	r.Use(middleware.RateLimit(rateLimiter))
	r.Use(middleware.TrustRemote) // tag every HTTP request as untrusted-by-default

	// Public paths (no auth required)
	publicPaths := []string{
		cfg.Server.ContextPath + "/v1/auth/",
		cfg.Server.ContextPath + "/health",
		cfg.Server.ContextPath + "/v1/diagnostics",
		cfg.Server.ContextPath + "/v1/models",
		"/health",
		"/metrics",
		"/swagger.json",
	}
	r.Use(middleware.JWTAuth(jwtService, publicPaths))
	r.Use(middleware.TenantExtractor(cfg.Tenant.DefaultID))

	// Prometheus metrics endpoint (no auth — listed in publicPaths above)
	r.Handle("/metrics", promhttp.Handler())

	// Routes
	r.Route(cfg.Server.ContextPath, func(r chi.Router) {
		// Health
		r.Get("/health", handler.Health)

		// Diagnostics ("doctor") — same engine as the brainsentry CLI.
		r.Get("/v1/diagnostics", diagnosticsHandler.Get)

		// Tier-based model routing
		r.Get("/v1/models", modelsHandler.List)
		r.Get("/v1/models/doctor", modelsHandler.Doctor)

		// Operator-only destructive endpoints (CLI-only by trust contract)
		r.Post("/v1/admin/wipe-embedding-cache", middleware.RequireLocalTrust(adminTrustHandler.WipeEmbeddingCache))

		// Eval capture/export (opt-in via BRAINSENTRY_EVAL_CAPTURE=1)
		r.Get("/v1/eval/candidates.ndjson", evalHandler.Export)
		r.Get("/v1/eval/candidates/stats", evalHandler.Stats)
		r.Post("/v1/eval/candidates/reset", evalHandler.Reset)

		// Auth
		r.Route("/v1/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
			r.Post("/logout", authHandler.Logout)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/demo", authHandler.DemoLogin)
			r.Get("/sso/authorize", ssoHandler.GetAuthURL)
			r.Post("/sso/callback", ssoHandler.Callback)
			r.Get("/sso/config", ssoHandler.GetConfig)
		})

		// Users (admin-only for create)
		r.Route("/v1/users", func(r chi.Router) {
			r.Get("/", userHandler.List)
			r.Get("/{id}", userHandler.GetByID)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Post("/", userHandler.Create)
		})

		// Tenants (admin-only for write operations)
		r.Route("/v1/tenants", func(r chi.Router) {
			r.Get("/", tenantHandler.List)
			r.Get("/{id}", tenantHandler.GetByID)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Post("/", tenantHandler.Create)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Put("/{id}", tenantHandler.Update)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Delete("/{id}", tenantHandler.Delete)
		})

		// Memories
		r.Route("/v1/memories", func(r chi.Router) {
			r.Post("/", memoryHandler.Create)
			r.Get("/", memoryHandler.List)
			r.Post("/search", memoryHandler.Search)
			r.Get("/by-category/{category}", memoryHandler.GetByCategory)
			r.Get("/by-importance/{importance}", memoryHandler.GetByImportance)
			r.Get("/{id}", memoryHandler.GetByID)
			r.Put("/{id}", memoryHandler.Update)
			r.Delete("/{id}", memoryHandler.Delete)
			r.Get("/{id}/versions", memoryHandler.Versions)
			r.Post("/{id}/feedback", memoryHandler.Feedback)
			r.Post("/{id}/flag", correctionHandler.Flag)
			r.Post("/{id}/review", correctionHandler.Review)
			r.Post("/{id}/rollback", correctionHandler.Rollback)
		})

		// Interception
		r.Post("/v1/intercept", interceptionHandler.Intercept)

		// Relationships
		r.Route("/v1/relationships", func(r chi.Router) {
			r.Get("/", relationshipHandler.List)
			r.Post("/", relationshipHandler.Create)
			r.Post("/bidirectional", relationshipHandler.CreateBidirectional)
			r.Get("/from/{memoryId}", relationshipHandler.GetFrom)
			r.Get("/to/{memoryId}", relationshipHandler.GetTo)
			r.Get("/between", relationshipHandler.GetBetween)
			r.Get("/{memoryId}/related", relationshipHandler.GetRelated)
			r.Put("/{relationshipId}/strength", relationshipHandler.UpdateStrength)
			r.Delete("/between", relationshipHandler.DeleteBetween)
			r.Delete("/{memoryId}", relationshipHandler.DeleteAll)
			r.Post("/{memoryId}/suggest", relationshipHandler.Suggest)
		})

		// Entity Graph (only if FalkorDB is available)
		if entityGraphHandler != nil {
			r.Route("/v1/entity-graph", func(r chi.Router) {
				r.Get("/memory/{memoryId}/entities", entityGraphHandler.GetEntitiesByMemory)
				r.Get("/memory/{memoryId}/relationships", entityGraphHandler.GetRelationshipsByMemory)
				r.Get("/search", entityGraphHandler.SearchEntities)
				r.Get("/knowledge-graph", entityGraphHandler.GetKnowledgeGraph)
				r.Post("/extract/{memoryId}", entityGraphHandler.ExtractEntities)
				r.Post("/extract-batch", entityGraphHandler.BatchExtract)
			})
		}

		// Audit Logs (spec path: /v1/audit/logs)
		r.Route("/v1/audit", func(r chi.Router) {
			r.Get("/logs", auditHandler.List)
			r.Get("/logs/by-event/{eventType}", auditHandler.ByEventType)
			r.Get("/logs/by-user/{userId}", auditHandler.ByUser)
			r.Get("/logs/by-session/{sessionId}", auditHandler.BySession)
			r.Get("/logs/recent", auditHandler.Recent)
			r.Get("/logs/by-date-range", auditHandler.ByDateRange)
			r.Get("/logs/stats", auditHandler.Stats)
			r.Get("/memory/{memoryId}/history", auditHandler.ByMemory)
		})

		// Stats
		r.Route("/v1/stats", func(r chi.Router) {
			r.Get("/overview", statsHandler.Overview)
			r.Get("/top-patterns", statsHandler.TopPatterns)
			r.Get("/health", statsHandler.HealthStats)
		})

		// Notes
		r.Route("/v1/notes", func(r chi.Router) {
			r.Get("/", noteTakingHandler.ListNotes)
			r.Post("/analyze", noteTakingHandler.AnalyzeSession)
			r.Get("/hindsight", noteTakingHandler.ListHindsight)
			r.Post("/hindsight", noteTakingHandler.CreateHindsight)
			r.Get("/session/{sessionId}", noteTakingHandler.GetSessionNotes)
			r.Get("/session/{sessionId}/hindsight", noteTakingHandler.GetSessionHindsight)
		})

		// Compression
		r.Route("/v1/compression", func(r chi.Router) {
			r.Post("/compress", compressionHandler.Compress)
			r.Get("/session/{sessionId}", compressionHandler.GetSessionSummaries)
			r.Get("/session/{sessionId}/latest", compressionHandler.GetLatestSummary)
		})

		// Sessions
		r.Route("/v1/sessions", func(r chi.Router) {
			r.Post("/", sessionHandler.Create)
			r.Get("/active", sessionHandler.ListActive)
			r.Get("/{id}", sessionHandler.Get)
			r.Post("/{id}/touch", sessionHandler.Touch)
			r.Post("/{id}/end", sessionHandler.End)
		})

		// Batch Import/Export
		r.Route("/v1/batch", func(r chi.Router) {
			r.Post("/import", batchHandler.Import)
			r.Get("/export", batchHandler.Export)
		})

		// Conflict Detection
		r.Route("/v1/conflicts", func(r chi.Router) {
			r.Post("/detect/{memoryId}", conflictHandler.DetectForMemory)
			r.Post("/scan", conflictHandler.ScanAll)
			r.Get("/near-duplicates", conflictHandler.NearDuplicates)
		})

		// Webhooks
		r.Route("/v1/webhooks", func(r chi.Router) {
			r.Post("/", webhookHandler.Register)
			r.Get("/", webhookHandler.List)
			r.Delete("/{id}", webhookHandler.Unregister)
			r.Get("/{id}/deliveries", webhookHandler.Deliveries)
		})

		// MCP (Model Context Protocol)
		r.Route("/v1/mcp", func(r chi.Router) {
			r.Post("/message", mcpHandler.HandleMessage)
			r.Post("/sse", mcpHandler.HandleSSE)
			r.Post("/batch", mcpHandler.HandleBatch)
		})

		// Profile
		if profileHandler != nil {
			r.Route("/v1/profile", func(r chi.Router) {
				r.Get("/", profileHandler.GetProfile)
				r.Get("/{userId}", profileHandler.GetProfileByUser)
			})
		}

		// NL Graph Query
		if nlQueryHandler != nil {
			r.Post("/v1/graph/nl-query", nlQueryHandler.Query)
		}

		// Graph Communities (Louvain)
		if communitiesHandler != nil {
			r.Get("/v1/graph/communities", communitiesHandler.DetectCommunities)
		}

		// Graph Views (global map, ego graph, bi-temporal timeline)
		if graphViewHandler != nil {
			r.Get("/v1/graph/global", graphViewHandler.Global)
			r.Get("/v1/graph/ego", graphViewHandler.Ego)
			r.Get("/v1/graph/timeline", graphViewHandler.Timeline)
		}

		// Reflection
		if reflectionHandler != nil {
			r.Post("/v1/reflect", reflectionHandler.RunReflection)
		}

		// Reconciliation
		if reconciliationHandler != nil {
			r.Post("/v1/reconcile", reconciliationHandler.Reconcile)
		}

		// Retrieval Planner (advanced search)
		if retrievalHandler != nil {
			r.Post("/v1/memories/plan-search", retrievalHandler.PlanSearch)
		}

		// Spreading Activation
		if activationHandler != nil {
			r.Post("/v1/memories/activate", activationHandler.Activate)
		}

		// Cross-Session
		if crossSessionHandler != nil {
			r.Get("/v1/sessions/{id}/events", crossSessionHandler.GetSessionEvents)
			r.Get("/v1/sessions/{id}/cross-context", crossSessionHandler.GetCrossContext)
		}

		// Connectors
		if connectorsHandler != nil {
			r.Route("/v1/connectors", func(r chi.Router) {
				r.Get("/", connectorsHandler.List)
				r.Post("/sync-all", connectorsHandler.SyncAll)
				r.Post("/{name}/sync", connectorsHandler.Sync)
			})
		}

		// Tasks
		if tasksHandler != nil {
			r.Route("/v1/tasks", func(r chi.Router) {
				r.Get("/metrics", tasksHandler.Metrics)
				r.Get("/pending", tasksHandler.Pending)
			})
		}

		// Consolidation
		r.Post("/v1/consolidate", consolidationHandler.Consolidate)

		// Benchmark (admin only)
		r.With(middleware.RequireRole(middleware.RoleAdmin)).Post("/v1/benchmark/run", benchmarkHandler.RunBenchmark)

		// Admin endpoints
		r.Route("/v1/admin", func(r chi.Router) {
			r.Use(middleware.RequireRole(middleware.RoleAdmin))
			r.Get("/circuit-breakers", adminHandler.GetCircuitBreakers)
			r.Get("/llm-metrics", adminHandler.GetLLMMetrics)
		})

		// PII Scanner
		r.Post("/v1/pii/scan", adminHandler.ScanPII)

		// Auto-Forget (admin only)
		r.With(middleware.RequireRole(middleware.RoleAdmin)).Post("/v1/auto-forget", autoForgetHandler.Run)

		// Semantic Memory Consolidation
		r.Post("/v1/semantic/consolidate", semanticMemoryHandler.Consolidate)

		// Actions & Leases (multi-agent coordination)
		r.Route("/v1/actions", func(r chi.Router) {
			r.Post("/", actionsHandler.Create)
			r.Get("/", actionsHandler.List)
			r.Get("/{id}", actionsHandler.Get)
			r.Put("/{id}/status", actionsHandler.UpdateStatus)
			r.Post("/{id}/lease", actionsHandler.AcquireLease)
			r.Delete("/{id}/lease", actionsHandler.ReleaseLease)
		})

		// Mesh Sync (P2P)
		r.Route("/v1/mesh", func(r chi.Router) {
			r.Post("/peers", meshHandler.RegisterPeer)
			r.Get("/peers", meshHandler.ListPeers)
			r.Post("/sync", meshHandler.Sync)
		})

		// P1-Cognee: Query Router (rule-based, LLM-free)
		r.Post("/v1/router/classify", queryRouterHandler.Classify)

		// P1-Cognee: Triplet Extraction (S,P,O from content)
		r.Post("/v1/triplets/extract", tripletHandler.Extract)

		// P1-Cognee: Cascade Entity Extraction (3-pass LLM)
		r.Post("/v1/cascade-extract", cascadeExtractionHandler.Extract)

		// P1-Cognee: Feedback Learning weight inspection
		r.Get("/v1/memories/{id}/feedback-weight", feedbackLearningHandler.GetWeight)

		// P2-Cognee: Semantic API (remember/recall/improve/forget)
		r.Post("/v1/remember", semanticAPIHandler.Remember)
		r.Post("/v1/recall", semanticAPIHandler.Recall)
		r.Post("/v1/improve", semanticAPIHandler.Improve)
		r.Post("/v1/forget", semanticAPIHandler.Forget)

		// P2-Cognee: AgentTrace (procedural memory)
		r.Route("/v1/traces", func(r chi.Router) {
			r.Post("/", agentTraceHandler.Record)
			r.Get("/", agentTraceHandler.List)
			r.Get("/stats", agentTraceHandler.Stats)
			r.Get("/{id}", agentTraceHandler.Get)
			r.Delete("/{id}", agentTraceHandler.Delete)
		})

		// P2-Cognee: NodeSet (multi-set grouping)
		r.Route("/v1/memories/{id}/sets", func(r chi.Router) {
			r.Get("/", nodeSetHandler.GetSets)
			r.Post("/", nodeSetHandler.AddToSet)
			r.Delete("/", nodeSetHandler.RemoveFromSet)
		})

		// P3-Cognee: Ontology
		r.Route("/v1/ontology", func(r chi.Router) {
			r.Get("/", ontologyHandler.Get)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Put("/", ontologyHandler.Set)
			r.Post("/resolve", ontologyHandler.Resolve)
		})

		// P3-Cognee: Session Cache
		r.Route("/v1/session-cache", func(r chi.Router) {
			r.Get("/", sessionCacheHandler.ListSessions)
			r.Post("/{sessionId}", sessionCacheHandler.Push)
			r.Get("/{sessionId}", sessionCacheHandler.List)
			r.Delete("/{sessionId}", sessionCacheHandler.Clear)
			r.Post("/{sessionId}/cognify", sessionCacheHandler.Cognify)
		})

		// P3-Cognee: Batch Search (multi-query parallel)
		r.Post("/v1/memories/batch-search", batchSearchHandler.Search)

		// Semantica: Decisions
		r.Route("/v1/decisions", func(r chi.Router) {
			r.Post("/", decisionHandler.Record)
			r.Get("/", decisionHandler.List)
			r.Post("/precedents", decisionHandler.SearchPrecedents)
			r.Get("/{id}", decisionHandler.Get)
			r.Get("/{id}/precedents", decisionHandler.Precedents)
			r.Get("/{id}/causal-chain", decisionHandler.CausalChain)
			r.Get("/{id}/influence", decisionHandler.Influence)
			r.Post("/{id}/supersede", decisionHandler.Supersede)
		})

		// Semantica: Policies
		r.Route("/v1/policies", func(r chi.Router) {
			r.Get("/", policyHandler.List)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Post("/", policyHandler.Create)
			r.Post("/enforce", policyHandler.EnforceOnDecision)
			r.Get("/{id}", policyHandler.Get)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Put("/{id}", policyHandler.Update)
			r.With(middleware.RequireRole(middleware.RoleAdmin)).Delete("/{id}", policyHandler.Delete)
		})

		// Semantica: Events
		r.Route("/v1/events", func(r chi.Router) {
			r.Post("/", eventHandler.Record)
			r.Get("/", eventHandler.List)
			r.Get("/stats", eventHandler.Stats)
			r.Post("/extract", eventHandler.Extract)
			r.Get("/{id}", eventHandler.Get)
			r.Delete("/{id}", eventHandler.Delete)
		})

		// Semantica: Reasoning (abductive + future engines)
		r.Post("/v1/reasoning/abduce", reasoningHandler.Abduce)

		// Semantica: W3C PROV-O export
		r.Get("/v1/export/provenance", provenanceHandler.Export)

		// Semantica: Bi-temporal memory query
		r.Get("/v1/memories/as-of", biTemporalHandler.AsOf)

		// Semantica: Coreference resolution (used before extraction)
		r.Post("/v1/extract/resolve-coreferences", coreferenceHandler.Resolve)
	})

	// Integration endpoints (service-to-service auth)
	r.Route("/api/v1/integration", func(r chi.Router) {
		r.Use(middleware.ServiceAuth)
		r.Post("/execution/start", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","message":"integration endpoint ready"}`))
		})
		r.Post("/execution/end", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","message":"integration endpoint ready"}`))
		})
	})

	// Also mount health at root for container probes
	r.Get("/health", handler.Health)

	// Swagger/OpenAPI spec
	r.Get("/swagger.json", handler.SwaggerSpec)

	// Server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	startupTime := time.Now()
	logger.Info("Brain Sentry Go started",
		"port", cfg.Server.Port,
		"startup_ms", time.Since(startupTime).Milliseconds(),
	)

	<-done
	logger.Info("shutting down...")

	// Stop background services
	learningService.Stop()
	sessionService.Stop()
	if taskScheduler != nil {
		taskScheduler.Stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}

// runRebuildAndExit executes the requested rebuild targets in-process and
// exits. Reached only when `--rebuild=<targets>` was passed. Trust is
// elevated to Local: the process is on the operator's host, so the same
// guarantee that protects HTTP-borne destructive ops is unnecessary here.
func runRebuildAndExit(logger *slog.Logger, svc *rebuild.Service, targetsCSV string, confirm bool) {
	targets := splitNonEmpty(targetsCSV, ",")
	if !confirm {
		fmt.Println("brainsentry rebuild — DRY RUN (re-run with --confirm-destructive to execute)")
		fmt.Println("would run:", strings.Join(targets, ", "))
		fmt.Println("registered targets:", strings.Join(svc.Targets(), ", "))
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(trust.WithLocal(context.Background()), 30*time.Minute)
	defer cancel()
	rep := svc.Run(ctx, targets)
	fmt.Printf("brainsentry rebuild — completed in %dms\n", rep.Duration.Milliseconds())
	for _, r := range rep.Results {
		mark := "PASS"
		if !r.OK {
			mark = "FAIL"
		}
		fmt.Printf("  [%s] %-12s touched=%d duration=%dms", mark, r.Target, r.Touched, r.Duration.Milliseconds())
		if r.Error != "" {
			fmt.Printf("  err=%s", r.Error)
		}
		fmt.Println()
	}
	if !rep.OK {
		logger.Error("rebuild failed", "targets", targets)
		os.Exit(1)
	}
	os.Exit(0)
}

func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
