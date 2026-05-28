import axios, { AxiosInstance, InternalAxiosRequestConfig, AxiosResponse } from "axios";

// Configuração base da API
const API_BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:8080/api";

// Tipos de resposta da API
export interface ApiResponse<T> {
  data: T;
  message?: string;
}

export interface ApiError {
  message: string;
  statusCode?: number;
  details?: unknown;
}

// Tipos específicos do domínio - alinhados com o backend
// New universal categories
export type MemoryCategory =
  | "INSIGHT"      // Patterns, best practices, preferences
  | "DECISION"     // Decisions (technical or business)
  | "WARNING"      // Anti-patterns, bugs, objections
  | "KNOWLEDGE"    // Domain/customer/product knowledge
  | "ACTION"       // Actions, optimizations, follow-ups
  | "CONTEXT"      // Context, integrations, history
  | "REFERENCE"    // Documentation, materials
  // Legacy categories (deprecated, for backward compatibility)
  | "PATTERN" | "ANTIPATTERN" | "DOMAIN" | "BUG" | "OPTIMIZATION" | "INTEGRATION";
export type ImportanceLevel = "CRITICAL" | "IMPORTANT" | "MINOR";

export interface Memory {
  id: string;
  tenantId?: string;
  content: string;
  summary: string;
  category: MemoryCategory | string;
  importance: ImportanceLevel | string;
  validationStatus?: string;
  metadata?: Record<string, unknown>;
  tags: string[];
  sourceType?: string;
  sourceReference?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt?: string;
  accessCount?: number;
  injectionCount?: number;
  helpfulCount?: number;
  embedding?: number[];
  memoryType?: string;
  emotionalWeight?: number;
  simHash?: string;
  validFrom?: string;
  validTo?: string;
  decayRate?: number;
  supersededBy?: string;
  decayedRelevance?: number;
}

export interface CreateMemoryRequest {
  content: string;
  summary: string;
  category?: MemoryCategory;
  importance?: ImportanceLevel;
  tags?: string[];
}

export interface UpdateMemoryRequest {
  content?: string;
  summary?: string;
  category?: MemoryCategory;
  importance?: ImportanceLevel;
  tags?: string[];
}

export interface MemoryListResponse {
  memories: Memory[];
  total: number;
  totalElements?: number;
  page: number;
  size: number;
  totalPages: number;
  hasNext?: boolean;
  hasPrevious?: boolean;
}

export interface SearchRequest {
  query: string;
  limit?: number;
}

export interface MemoryStats {
  totalMemories: number;
  memoriesByCategory: Record<string, number>;
  memoriesByImportance: Record<string, number>;
  requestsToday: number;
  injectionRate: number;
  avgLatencyMs: number;
  helpfulnessRate: number;
  totalInjections: number;
  activeMemories24h: number;
}

type SearchResponse = Memory[] | { results?: Memory[]; total?: number; searchTimeMs?: number };
type RawMemoryListResponse = Omit<MemoryListResponse, "total"> & { total?: number };

function normalizeMemoryListResponse(data: RawMemoryListResponse): MemoryListResponse {
  const total = data.total ?? data.totalElements ?? data.memories?.length ?? 0;
  return {
    ...data,
    total,
    totalElements: data.totalElements ?? total,
  };
}

// ===== Cognee P1-P3 types =====

// Semantic API
export interface RememberRequest {
  text: string;
  title?: string;
  sessionId?: string;
  sets?: string[];
  tags?: string[];
  category?: MemoryCategory;
  importance?: ImportanceLevel;
}

export interface RememberResponse {
  memoryId: string;
  sets?: string[];
  title?: string;
  createdAt: string;
}

export interface RecallRequest {
  query: string;
  set?: string;
  limit?: number;
  tags?: string[];
}

export interface RecallResult {
  memoryId: string;
  content: string;
  summary?: string;
  relevance: number;
  category?: string;
  feedbackWeight: number;
  createdAt: string;
  sets?: string[];
}

export interface RecallResponse {
  query: string;
  strategy: string;
  results: RecallResult[];
  total: number;
}

export interface ImproveRequest {
  sessionId?: string;
  dryRun?: boolean;
}

export interface ImproveResponse {
  autoForgetResult?: {
    ttl_expired: number;
    contradictions: number;
    low_value: number;
    total_deleted: number;
    dry_run: boolean;
    deleted_ids?: string[];
  };
  message: string;
}

export interface ForgetRequest {
  memoryId?: string;
  set?: string;
  query?: string;
}

export interface ForgetResponse {
  deletedIds: string[];
  count: number;
  message: string;
}

// Query Router
export type SearchStrategy =
  | "LEXICAL" | "SEMANTIC" | "GRAPH" | "TEMPORAL"
  | "ENTITY" | "CODING" | "CYPHER" | "HYBRID";

export interface RouterDecision {
  strategy: SearchStrategy;
  confidence: number;
  scores?: Record<SearchStrategy, number>;
  matchedPatterns?: string[];
  fallback: boolean;
}

// Agent Traces
export interface AgentTrace {
  id: string;
  tenantId: string;
  sessionId?: string;
  agentId?: string;
  originFunction: string;
  withMemory: boolean;
  memoryQuery?: string;
  methodParams?: Record<string, any>;
  methodReturn?: any;
  memoryContext?: string;
  status: "success" | "error";
  errorMessage?: string;
  text: string;
  durationMs: number;
  createdAt: string;
  memoryIds?: string[];
  belongsToSets?: string[];
}

export interface AgentTraceFilter {
  sessionId?: string;
  agentId?: string;
  status?: "success" | "error";
  set?: string;
  limit?: number;
}

export interface AgentTraceListResponse {
  count: number;
  traces: AgentTrace[];
}

export interface AgentTraceStats {
  total: number;
  success: number;
  errors: number;
  withMemory: number;
  avgDurationMs: number;
  errorRate: number;
}

// Triplets
export interface Triplet {
  id: string;
  memoryId: string;
  subject: string;
  predicate: string;
  object: string;
  text: string;
  confidence: number;
  createdAt: string;
  feedbackWeight: number;
}

export interface TripletExtractResponse {
  memoryId: string;
  count: number;
  triplets: Triplet[];
}

// Cascade Extraction
export interface ExtractedEntity {
  name: string;
  type: string;
  properties?: Record<string, string>;
}

export interface ExtractedRelationship {
  source: string;
  target: string;
  type: string;
  properties?: Record<string, string>;
}

export interface CascadeExtractResponse {
  entities: ExtractedEntity[];
  relationships: ExtractedRelationship[];
  passCount: number;
}

// Feedback Learning
export interface FeedbackWeightResponse {
  memoryId: string;
  helpfulCount: number;
  notHelpfulCount: number;
  feedbackWeight: number;
  alpha: number;
}

// Ontology
export interface OntologyResolveResponse {
  input: string;
  matched: boolean;
  canonical: string;
  type: string;
}

// Batch Search
export interface BatchSearchRequest {
  queries: string[];
  limit?: number;
  tags?: string[];
}

export interface BatchScore {
  memoryId: string;
  summary?: string;
  category?: string;
  perQuery: number[];
  matchedQueries: number[];
  mean: number;
  max: number;
}

export interface BatchSearchResponse {
  queries: string[];
  results: BatchScore[];
  searchTimeMs: number;
}

// Session Cache
export interface SessionInteraction {
  id: string;
  query: string;
  response: string;
  memoryIds?: string[];
  createdAt: string;
  metadata?: Record<string, string>;
}

export interface SessionCacheListResponse {
  sessionId: string;
  count: number;
  interactions: SessionInteraction[];
}

export interface CognifyResult {
  sessionId: string;
  interactions: number;
  memoriesCreated: string[];
}

// Actions & Leases
export type ActionStatus = "pending" | "in_progress" | "blocked" | "completed" | "cancelled";

export interface Action {
  id: string;
  title: string;
  description: string;
  status: ActionStatus;
  priority: number;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
  assignedTo?: string;
  parentId?: string;
  tags?: string[];
  dependsOn?: string[];
}

export interface CreateActionRequest {
  title: string;
  description: string;
  createdBy: string;
  priority: number;
  tags?: string[];
  parentId?: string;
  dependsOn?: string[];
}

export interface Lease {
  actionId: string;
  heldBy: string;
  acquiredAt: string;
  expiresAt: string;
}

// Mesh Sync
export interface MeshPeer {
  id: string;
  url: string;
  sharedScopes: string[];
  lastSyncAt?: string;
  status?: string;
}

export interface MeshSyncResult {
  peerId: string;
  scope: string;
  sent?: number;
  received?: number;
  merged?: number;
  error?: string;
}

// Interceptador para adicionar headers de autenticação
const authRequestInterceptor = (config: InternalAxiosRequestConfig): InternalAxiosRequestConfig => {
  // Adicionar tenant ID se disponível
  const tenantId = localStorage.getItem("tenant_id") || "a9f814d2-4dae-41f3-851b-8aa3d4706561";

  // Adicionar token JWT se disponível
  const token = localStorage.getItem("brain_sentry_token");

  if (config.headers) {
    config.headers["X-Tenant-ID"] = tenantId;
    if (token) {
      config.headers["Authorization"] = `Bearer ${token}`;
    }
  }
  return config;
};

// Interceptador para tratamento de erros
const errorResponseInterceptor = (error: unknown): Promise<ApiError> => {
  const apiError: ApiError = {
    message: "Erro desconhecido",
    statusCode: 0,
    details: error,
  };

  if (axios.isAxiosError(error)) {
    const responseData = error.response?.data as { message?: string } | undefined;
    apiError.message = responseData?.message || error.message || "Erro desconhecido";
    apiError.statusCode = error.response?.status;
    apiError.details = error.response?.data;
  } else if (error instanceof Error) {
    apiError.message = error.message;
  }

  return Promise.reject(apiError);
};

// Interceptador para tratamento de respostas de sucesso
const successResponseInterceptor = (response: AxiosResponse): AxiosResponse => {
  return response;
};

// Classe principal do API Client
class ApiClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      timeout: 30000,
      headers: {
        "Content-Type": "application/json",
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors(): void {
    this.client.interceptors.request.use(authRequestInterceptor);
    this.client.interceptors.response.use(successResponseInterceptor, errorResponseInterceptor);
  }

  // Memory endpoints - alinhados com MemoryController do backend
  async getMemories(page: number = 0, size: number = 20): Promise<MemoryListResponse> {
    const response = await this.client.get<MemoryListResponse>("/v1/memories", {
      params: { page, size },
    });
    return normalizeMemoryListResponse(response.data);
  }

  async getMemory(id: string): Promise<Memory> {
    const response = await this.client.get<Memory>(`/v1/memories/${id}`);
    return response.data;
  }

  async createMemory(data: CreateMemoryRequest): Promise<Memory> {
    const response = await this.client.post<Memory>("/v1/memories", data);
    return response.data;
  }

  async updateMemory(id: string, data: UpdateMemoryRequest): Promise<Memory> {
    const response = await this.client.put<Memory>(`/v1/memories/${id}`, data);
    return response.data;
  }

  async deleteMemory(id: string): Promise<void> {
    await this.client.delete(`/v1/memories/${id}`);
  }

  async searchMemories(query: string, limit: number = 10): Promise<Memory[]> {
    const response = await this.client.post<SearchResponse>("/v1/memories/search", {
      query,
      limit,
    });
    return Array.isArray(response.data) ? response.data : response.data.results || [];
  }

  async getMemoriesByCategory(category: string): Promise<Memory[]> {
    const response = await this.client.get<Memory[]>(`/v1/memories/by-category/${category}`);
    return response.data;
  }

  async getMemoriesByImportance(importance: string): Promise<Memory[]> {
    const response = await this.client.get<Memory[]>(`/v1/memories/by-importance/${importance}`);
    return response.data;
  }

  async getRelatedMemories(id: string, depth: number = 2): Promise<Memory[]> {
    const response = await this.client.get<Memory[]>(`/v1/memories/${id}/related`, {
      params: { depth },
    });
    return response.data;
  }

  async recordFeedback(id: string, helpful: boolean): Promise<void> {
    await this.client.post(`/v1/memories/${id}/feedback`, null, {
      params: { helpful },
    });
  }

  // Stats endpoints
  async getStats(): Promise<MemoryStats> {
    const response = await this.client.get<MemoryStats>("/v1/stats/overview");
    return response.data;
  }

  // Health check
  async healthCheck(): Promise<{ status: string; timestamp: string }> {
    const response = await this.client.get("/v1/stats/health");
    return response.data;
  }

  // Profile
  async getProfile(): Promise<any> {
    const response = await this.client.get("/v1/profile");
    return response.data;
  }

  // NL Graph Query
  async nlQuery(question: string): Promise<any> {
    const response = await this.client.post("/v1/graph/nl-query", { question });
    return response.data;
  }

  // Reflection
  async runReflection(): Promise<any> {
    const response = await this.client.post("/v1/reflect");
    return response.data;
  }

  // Reconciliation
  async reconcileFacts(content: string, sessionId?: string): Promise<any> {
    const response = await this.client.post("/v1/reconcile", { content, sessionId });
    return response.data;
  }

  // Retrieval Planner
  async planSearch(query: string, limit: number = 10): Promise<any> {
    const response = await this.client.post("/v1/memories/plan-search", { query, limit });
    return response.data;
  }

  // Spreading Activation
  async activateMemories(seedIds: string[], seedActivations?: number[]): Promise<any> {
    const response = await this.client.post("/v1/memories/activate", { seedIds, seedActivations });
    return response.data;
  }

  // Graph Communities
  async getCommunities(): Promise<any> {
    const response = await this.client.get("/v1/graph/communities");
    return response.data;
  }

  // Interception
  async intercept(prompt: string, sessionId?: string): Promise<any> {
    const response = await this.client.post("/v1/intercept", { prompt, sessionId });
    return response.data;
  }

  // Compression
  async compress(messages: any[], options?: any): Promise<any> {
    const response = await this.client.post("/v1/compression/compress", { messages, ...options });
    return response.data;
  }

  // Connectors
  async getConnectors(): Promise<any> {
    const response = await this.client.get("/v1/connectors");
    return response.data;
  }

  async syncConnector(name: string): Promise<any> {
    const response = await this.client.post(`/v1/connectors/${name}/sync`);
    return response.data;
  }

  // Tasks
  async getTaskMetrics(): Promise<any> {
    const response = await this.client.get("/v1/tasks/metrics");
    return response.data;
  }

  // Consolidation
  async consolidate(similarityThreshold: number = 0.85): Promise<any> {
    const response = await this.client.post("/v1/consolidate", { similarityThreshold });
    return response.data;
  }

  // Benchmark
  async runBenchmark(queryCount: number = 10, k: number = 10): Promise<any> {
    const response = await this.client.post("/v1/benchmark/run", { queryCount, k });
    return response.data;
  }

  // Admin
  async getCircuitBreakers(): Promise<any> {
    const response = await this.client.get("/v1/admin/circuit-breakers");
    return response.data;
  }

  async getLLMMetrics(): Promise<any> {
    const response = await this.client.get("/v1/admin/llm-metrics");
    return response.data;
  }

  async scanPII(text: string): Promise<any> {
    const response = await this.client.post("/v1/pii/scan", { text });
    return response.data;
  }

  // Memory Versions
  async getMemoryVersions(id: string): Promise<any> {
    const response = await this.client.get(`/v1/memories/${id}/versions`);
    return response.data;
  }

  // Memory Correction
  async flagMemory(id: string, reason: string): Promise<any> {
    const response = await this.client.post(`/v1/memories/${id}/flag`, { reason });
    return response.data;
  }

  async reviewCorrection(id: string, action: string): Promise<any> {
    const response = await this.client.post(`/v1/memories/${id}/review`, { action });
    return response.data;
  }

  async rollbackMemory(id: string, version: number): Promise<any> {
    const response = await this.client.post(`/v1/memories/${id}/rollback`, { version });
    return response.data;
  }

  // Batch
  async importBatch(memories: any[]): Promise<any> {
    const response = await this.client.post("/v1/batch/import", { memories });
    return response.data;
  }

  async exportBatch(): Promise<any> {
    const response = await this.client.get("/v1/batch/export");
    return response.data;
  }

  // Webhooks
  async listWebhooks(): Promise<any> {
    const response = await this.client.get("/v1/webhooks");
    return response.data;
  }

  async createWebhook(url: string, events: string[]): Promise<any> {
    const response = await this.client.post("/v1/webhooks", { url, events });
    return response.data;
  }

  async deleteWebhook(id: string): Promise<void> {
    await this.client.delete(`/v1/webhooks/${id}`);
  }

  // Conflicts
  async detectConflicts(memoryId: string): Promise<any> {
    const response = await this.client.post(`/v1/conflicts/detect/${memoryId}`);
    return response.data;
  }

  async scanConflicts(): Promise<any> {
    const response = await this.client.post("/v1/conflicts/scan");
    return response.data;
  }

  // Notes
  async getNotes(): Promise<any> {
    const response = await this.client.get("/v1/notes");
    return response.data;
  }

  async getHindsightNotes(): Promise<any> {
    const response = await this.client.get("/v1/notes/hindsight");
    return response.data;
  }

  async analyzeSession(sessionId: string): Promise<any> {
    const response = await this.client.post("/v1/notes/analyze", { sessionId });
    return response.data;
  }

  // Sessions
  async getSessionEvents(sessionId: string): Promise<any> {
    const response = await this.client.get(`/v1/sessions/${sessionId}/events`);
    return response.data;
  }

  // Knowledge Graph
  async getKnowledgeGraph(limit: number = 100): Promise<any> {
    const response = await this.client.get("/v1/entity-graph/knowledge-graph", {
      params: { limit },
    });
    return response.data;
  }

  // Audit Logs
  async getAuditLogs(limit: number = 100): Promise<any> {
    const response = await this.client.get("/v1/audit-logs", {
      params: { limit },
    });
    return response.data;
  }

  // ===== Cognee P1-P3 endpoints =====

  // Semantic API
  async remember(req: RememberRequest): Promise<RememberResponse> {
    const response = await this.client.post<RememberResponse>("/v1/remember", req);
    return response.data;
  }

  async recall(req: RecallRequest): Promise<RecallResponse> {
    const response = await this.client.post<RecallResponse>("/v1/recall", req);
    return response.data;
  }

  async improve(req: ImproveRequest = {}): Promise<ImproveResponse> {
    const response = await this.client.post<ImproveResponse>("/v1/improve", req);
    return response.data;
  }

  async forget(req: ForgetRequest): Promise<ForgetResponse> {
    const response = await this.client.post<ForgetResponse>("/v1/forget", req);
    return response.data;
  }

  // Query Router (rule-based, LLM-free)
  async classifyQuery(query: string): Promise<RouterDecision> {
    const response = await this.client.post<RouterDecision>("/v1/router/classify", { query });
    return response.data;
  }

  // Agent Traces
  async listAgentTraces(params: AgentTraceFilter = {}): Promise<AgentTraceListResponse> {
    const response = await this.client.get<AgentTraceListResponse>("/v1/traces", { params });
    return response.data;
  }

  async getAgentTraceStats(): Promise<AgentTraceStats> {
    const response = await this.client.get<AgentTraceStats>("/v1/traces/stats");
    return response.data;
  }

  async recordAgentTrace(req: Record<string, any>): Promise<AgentTrace> {
    const response = await this.client.post<AgentTrace>("/v1/traces", req);
    return response.data;
  }

  // Batch search (multi-query parallel)
  async batchSearch(req: BatchSearchRequest): Promise<BatchSearchResponse> {
    const response = await this.client.post<BatchSearchResponse>("/v1/memories/batch-search", req);
    return response.data;
  }

  // Session Cache
  async listSessionCaches(): Promise<{ count: number; sessions: string[] }> {
    const response = await this.client.get("/v1/session-cache");
    return response.data;
  }

  async getSessionCache(sessionId: string, limit: number = 20): Promise<SessionCacheListResponse> {
    const response = await this.client.get<SessionCacheListResponse>(`/v1/session-cache/${sessionId}`, {
      params: { limit },
    });
    return response.data;
  }

  async pushSessionCache(sessionId: string, interaction: Record<string, any>): Promise<void> {
    await this.client.post(`/v1/session-cache/${sessionId}`, interaction);
  }

  async clearSessionCache(sessionId: string): Promise<void> {
    await this.client.delete(`/v1/session-cache/${sessionId}`);
  }

  async cognifySessionCache(sessionId: string, clearAfter: boolean = false): Promise<CognifyResult> {
    const response = await this.client.post<CognifyResult>(
      `/v1/session-cache/${sessionId}/cognify`,
      null,
      { params: { clear: clearAfter ? "true" : "false" } }
    );
    return response.data;
  }

  async setOntology(ontology: Record<string, any>): Promise<any> {
    const response = await this.client.put("/v1/ontology", ontology);
    return response.data;
  }

  // Actions (multi-agent coordination)
  async listActions(status?: string): Promise<Action[]> {
    const response = await this.client.get<Action[]>("/v1/actions", {
      params: status ? { status } : {},
    });
    return response.data || [];
  }

  async createAction(req: CreateActionRequest): Promise<Action> {
    const response = await this.client.post<Action>("/v1/actions", req);
    return response.data;
  }

  async getAction(id: string): Promise<Action> {
    const response = await this.client.get<Action>(`/v1/actions/${id}`);
    return response.data;
  }

  async updateActionStatus(id: string, status: string): Promise<Action> {
    const response = await this.client.put<Action>(`/v1/actions/${id}/status`, { status });
    return response.data;
  }

  async acquireLease(id: string, agentId: string, ttlMinutes: number = 10): Promise<Lease> {
    const response = await this.client.post<Lease>(`/v1/actions/${id}/lease`, {
      agentId,
      ttlMinutes,
    });
    return response.data;
  }

  async releaseLease(id: string, agentId: string, completed: boolean = false): Promise<void> {
    await this.client.delete(`/v1/actions/${id}/lease`, {
      data: { agentId, completed },
    });
  }

  // Mesh (P2P sync)
  async listMeshPeers(): Promise<MeshPeer[]> {
    const response = await this.client.get<MeshPeer[]>("/v1/mesh/peers");
    return response.data || [];
  }

  async registerMeshPeer(peer: MeshPeer): Promise<void> {
    await this.client.post("/v1/mesh/peers", peer);
  }

  async meshSync(scope: string, items: any): Promise<MeshSyncResult[]> {
    const response = await this.client.post<MeshSyncResult[]>("/v1/mesh/sync", { scope, items });
    return response.data || [];
  }

  // Triplets
  async extractTriplets(content: string, memoryId?: string): Promise<TripletExtractResponse> {
    const response = await this.client.post<TripletExtractResponse>("/v1/triplets/extract", {
      content,
      memoryId,
    });
    return response.data;
  }

  // Cascade entity extraction
  async cascadeExtract(content: string): Promise<CascadeExtractResponse> {
    const response = await this.client.post<CascadeExtractResponse>("/v1/cascade-extract", { content });
    return response.data;
  }

  // Feedback Learning
  async getFeedbackWeight(memoryId: string): Promise<FeedbackWeightResponse> {
    const response = await this.client.get<FeedbackWeightResponse>(
      `/v1/memories/${memoryId}/feedback-weight`
    );
    return response.data;
  }

  // NodeSets
  async getMemorySets(memoryId: string): Promise<{ memoryId: string; sets: string[] }> {
    const response = await this.client.get(`/v1/memories/${memoryId}/sets`);
    return response.data;
  }

  async addMemorySets(memoryId: string, sets: string[]): Promise<{ memoryId: string; sets: string[] }> {
    const response = await this.client.post(`/v1/memories/${memoryId}/sets`, { sets });
    return response.data;
  }

  async removeMemorySets(memoryId: string, sets: string[]): Promise<{ memoryId: string; sets: string[] }> {
    const response = await this.client.delete(`/v1/memories/${memoryId}/sets`, { data: { sets } });
    return response.data;
  }

  // Ontology
  async getOntology(): Promise<any> {
    const response = await this.client.get("/v1/ontology");
    return response.data;
  }

  async resolveOntologyEntity(name: string): Promise<OntologyResolveResponse> {
    const response = await this.client.post<OntologyResolveResponse>("/v1/ontology/resolve", { name });
    return response.data;
  }

  // -------- Semantica-inspired: Decisions --------
  async recordDecision(req: RecordDecisionRequest): Promise<Decision> {
    const { data } = await this.client.post<Decision>("/v1/decisions", req);
    return data;
  }

  async listDecisions(params: {
    category?: string;
    agentId?: string;
    sessionId?: string;
    outcome?: string;
    as_of?: string;
    limit?: number;
    offset?: number;
  } = {}): Promise<{ count: number; decisions: Decision[] }> {
    const { data } = await this.client.get("/v1/decisions", { params });
    return data;
  }

  async getDecision(id: string): Promise<Decision> {
    const { data } = await this.client.get<Decision>(`/v1/decisions/${id}`);
    return data;
  }

  async findDecisionPrecedents(id: string, limit = 5): Promise<{ count: number; precedents: DecisionPrecedent[] }> {
    const { data } = await this.client.get(`/v1/decisions/${id}/precedents`, { params: { limit } });
    return data;
  }

  async searchDecisionPrecedents(req: { category: string; scenario?: string; limit?: number }): Promise<{ count: number; precedents: DecisionPrecedent[] }> {
    const { data } = await this.client.post("/v1/decisions/precedents", req);
    return data;
  }

  async getDecisionCausalChain(id: string, maxDepth = 5): Promise<{ count: number; chain: CausalNode[] }> {
    const { data } = await this.client.get(`/v1/decisions/${id}/causal-chain`, { params: { maxDepth } });
    return data;
  }

  async getDecisionInfluence(id: string): Promise<InfluenceReport> {
    const { data } = await this.client.get<InfluenceReport>(`/v1/decisions/${id}/influence`);
    return data;
  }

  async supersedeDecision(id: string, newId: string): Promise<void> {
    await this.client.post(`/v1/decisions/${id}/supersede`, { newId });
  }

  // -------- Policies --------
  async listPolicies(): Promise<{ count: number; policies: Policy[] }> {
    const { data } = await this.client.get("/v1/policies");
    return data;
  }

  async getPolicy(id: string): Promise<Policy> {
    const { data } = await this.client.get<Policy>(`/v1/policies/${id}`);
    return data;
  }

  async createPolicy(req: CreatePolicyRequest): Promise<Policy> {
    const { data } = await this.client.post<Policy>("/v1/policies", req);
    return data;
  }

  async updatePolicy(id: string, req: CreatePolicyRequest): Promise<Policy> {
    const { data } = await this.client.put<Policy>(`/v1/policies/${id}`, req);
    return data;
  }

  async deletePolicy(id: string): Promise<void> {
    await this.client.delete(`/v1/policies/${id}`);
  }

  async enforcePolicy(decisionId: string): Promise<{ decision: Decision; violations: PolicyViolation[]; compliant: boolean }> {
    const { data } = await this.client.post("/v1/policies/enforce", { decisionId });
    return data;
  }

  // -------- Events --------
  async recordEvent(req: RecordEventRequest): Promise<EventRecord> {
    const { data } = await this.client.post<EventRecord>("/v1/events", req);
    return data;
  }

  async listEvents(params: {
    eventType?: string;
    entityId?: string;
    from?: string;
    to?: string;
    limit?: number;
  } = {}): Promise<{ count: number; events: EventRecord[] }> {
    const { data } = await this.client.get("/v1/events", { params });
    return data;
  }

  async getEventStats(): Promise<Record<string, number>> {
    const { data } = await this.client.get("/v1/events/stats");
    return data;
  }

  async extractEvents(content: string, sourceMemoryId?: string): Promise<{ count: number; events: EventRecord[] }> {
    const { data } = await this.client.post("/v1/events/extract", { content, sourceMemoryId });
    return data;
  }

  async deleteEvent(id: string): Promise<void> {
    await this.client.delete(`/v1/events/${id}`);
  }

  // -------- Reasoning --------
  async abduceReasoning(req: { decisionId: string; question?: string; maxHypotheses?: number }): Promise<AbductionResult> {
    const { data } = await this.client.post<AbductionResult>("/v1/reasoning/abduce", req);
    return data;
  }

  // -------- Bi-temporal --------
  async getMemoriesAsOf(at: string, limit = 100): Promise<{ count: number; asOf: string; memories: Memory[] }> {
    const { data } = await this.client.get("/v1/memories/as-of", { params: { at, limit } });
    return data;
  }

  // -------- PROV-O export --------
  async exportProvenance(format: "turtle" | "jsonld" = "turtle"): Promise<string | Record<string, unknown>> {
    const { data } = await this.client.get("/v1/export/provenance", {
      params: { format },
      responseType: format === "jsonld" ? "json" : "text",
      transformResponse: [(v) => v],
    });
    return data;
  }

  // -------- Coreference --------
  async resolveCoreferences(content: string): Promise<{ original: string; resolved: string; resolutions?: Record<string, string> }> {
    const { data } = await this.client.post("/v1/extract/resolve-coreferences", { content });
    return data;
  }

  // -------- Graph Views --------
  async getGraphGlobal(params: {
    limit?: number;
    category?: string;
    importance?: string;
    communities?: boolean;
  } = {}): Promise<GraphResponse> {
    const { data } = await this.client.get("/v1/graph/global", {
      params: {
        limit: params.limit,
        category: params.category,
        importance: params.importance,
        communities: params.communities === false ? "false" : undefined,
      },
    });
    return data;
  }

  async getGraphEgo(memoryId: string, hops = 2, limit = 30): Promise<GraphResponse> {
    const { data } = await this.client.get("/v1/graph/ego", {
      params: { memoryId, hops, limit },
    });
    return data;
  }

  async getGraphTimeline(params: {
    from?: string;
    to?: string;
    limit?: number;
  } = {}): Promise<GraphResponse> {
    const { data } = await this.client.get("/v1/graph/timeline", { params });
    return data;
  }

  // -------- Diagnostics ("doctor") --------
  async getDiagnostics(): Promise<DiagnosticsReport> {
    const { data } = await this.client.get("/v1/diagnostics");
    return data;
  }

  // -------- Models (tier routing) --------
  async getModelsSnapshot(): Promise<{ snapshot: ModelResolveResult[] }> {
    const { data } = await this.client.get("/v1/models");
    return data;
  }

  async getModelsDoctor(): Promise<ModelsDoctorReport> {
    const { data } = await this.client.get("/v1/models/doctor");
    return data;
  }

  // Getter para o cliente axios bruto (para casos específicos)
  get axiosInstance(): AxiosInstance {
    return this.client;
  }
}

// -------- Diagnostics DTOs --------

export type DiagnosticsStatus = "ok" | "warn" | "fail" | "skip";
export type DiagnosticsSeverity = "info" | "warning" | "critical";

export interface DiagnosticsCheck {
  name: string;
  status: DiagnosticsStatus;
  severity: DiagnosticsSeverity;
  message: string;
  detail?: string;
  hint?: string;
  duration_ms: number;
}

export interface DiagnosticsReport {
  status: DiagnosticsStatus;
  generated_at: string;
  duration_ms: number;
  checks: DiagnosticsCheck[];
  summary: { ok: number; warn: number; fail: number; skip: number };
}

// -------- Models (tier routing) DTOs --------

export type ModelTier = "utility" | "reasoning" | "deep" | "subagent";
export type ModelFailureKind =
  | ""
  | "model_not_found"
  | "auth"
  | "rate_limit"
  | "network"
  | "timeout"
  | "invalid_request"
  | "unknown";

export interface ModelResolveResult {
  tier: ModelTier;
  model: string;
  source: string;
}

export interface ModelProbeResult {
  tier: ModelTier;
  model: string;
  ok: boolean;
  failure?: ModelFailureKind;
  duration_ms: number;
  detail?: string;
  hint?: string;
}

export interface ModelsDoctorReport {
  generated_at: string;
  duration_ms: number;
  ok: boolean;
  results: ModelProbeResult[];
}

// -------- Semantica DTOs --------

export type DecisionOutcome = "approved" | "rejected" | "deferred" | "pending";

export interface Decision {
  id: string;
  tenantId: string;
  category: string;
  scenario: string;
  reasoning: string;
  outcome: DecisionOutcome;
  confidence: number;
  agentId?: string;
  sessionId?: string;
  parentDecisionId?: string;
  entityIds?: string[];
  memoryIds?: string[];
  policyViolations?: string[];
  metadata?: Record<string, unknown>;
  createdAt: string;
  validFrom?: string;
  validUntil?: string;
  recordedAt: string;
  supersededBy?: string;
}

export interface DecisionPrecedent {
  decision: Decision;
  similarity: number;
}

export interface CausalNode {
  decision: Decision;
  depth: number;
  relation: "target" | "ancestor" | "descendant";
}

export interface InfluenceReport {
  descendants: number;
  maxDepth: number;
  supersedeCount: number;
  agreementRate: number;
  categoryEchoes: number;
}

export interface RecordDecisionRequest {
  category: string;
  scenario: string;
  reasoning: string;
  outcome?: DecisionOutcome;
  confidence?: number;
  agentId?: string;
  sessionId?: string;
  parentDecisionId?: string;
  entityIds?: string[];
  memoryIds?: string[];
  validFrom?: string;
  validUntil?: string;
  metadata?: Record<string, unknown>;
}

export type PolicySeverity = "info" | "warning" | "error" | "critical";
export type PolicyRuleType =
  | "min_confidence"
  | "requires_memory"
  | "requires_entity"
  | "forbidden_outcome"
  | "requires_reasoning"
  | "category_blocked";

export interface Policy {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  category: string;
  severity: PolicySeverity;
  ruleType: PolicyRuleType;
  ruleConfig: Record<string, unknown>;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  version: number;
}

export interface PolicyViolation {
  policyId: string;
  policyName: string;
  severity: PolicySeverity;
  message: string;
}

export interface CreatePolicyRequest {
  name: string;
  description?: string;
  category: string;
  severity?: PolicySeverity;
  ruleType: PolicyRuleType;
  ruleConfig?: Record<string, unknown>;
  enabled?: boolean;
}

export interface EventParticipant {
  entityId: string;
  role?: string;
  label?: string;
}

export interface EventRecord {
  id: string;
  tenantId: string;
  eventType: string;
  title: string;
  description: string;
  occurredAt: string;
  participants: EventParticipant[];
  attributes?: Record<string, unknown>;
  sourceMemoryId?: string;
  createdAt: string;
}

export interface RecordEventRequest {
  eventType: string;
  title?: string;
  description?: string;
  occurredAt?: string;
  participants?: EventParticipant[];
  attributes?: Record<string, unknown>;
  sourceMemoryId?: string;
}

export interface AbductionHypothesis {
  cause: string;
  confidence: number;
  evidence?: string[];
  memoryIds?: string[];
  entityIds?: string[];
}

export interface AbductionResult {
  decision: Decision;
  question: string;
  hypotheses: AbductionHypothesis[];
  evidenceUsed: number;
  model?: string;
}

// -------- Graph Views DTOs --------

export interface GraphNode {
  id: string;
  label: string;
  category?: string;
  importance?: string;
  communityId: number;
  accessCount?: number;
  helpfulCount?: number;
  notHelpfulCount?: number;
  emotionalWeight?: number;
  createdAt: string;
  validFrom?: string;
  validTo?: string;
  recordedAt: string;
  supersededBy?: string;
  tags?: string[];
  hopDistance?: number;
  score?: number;
}

export interface GraphEdge {
  source: string;
  target: string;
  type?: string;
  strength?: number;
}

export interface GraphCommunity {
  id: number;
  memberIds: string[];
  size: number;
  density: number;
}

export interface GraphResponse {
  nodes: GraphNode[];
  edges: GraphEdge[];
  communities?: GraphCommunity[];
  modularity?: number;
  tenantId?: string;
  total: number;
}

// Instância singleton
export const api = new ApiClient();

// Funções auxiliares para tratamento de erros
export function isApiError(error: unknown): error is ApiError {
  return typeof error === "object" && error !== null && "message" in error;
}

export function getErrorMessage(error: unknown): string {
  if (isApiError(error)) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Erro desconhecido";
}
