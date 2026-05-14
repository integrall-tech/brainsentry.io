export const DEMO_EMAIL = "demo@example.com";
export const DEMO_PASSWORD = "demo123";
export const DEFAULT_TENANT_ID = "a9f814d2-4dae-41f3-851b-8aa3d4706561";

export const API_BASE = "http://localhost:8081/api";

export const ROUTES = {
  login: "/login",
  dashboard: "/app/dashboard",
  memories: "/app/memories",
  search: "/app/search",
  relationships: "/app/relationships",
  timeline: "/app/timeline",
  audit: "/app/audit",
  users: "/app/users",
  tenants: "/app/tenants",
  configuration: "/app/configuration",
  analytics: "/app/analytics",
  profile: "/app/profile",
  playground: "/app/playground",
  connectors: "/app/connectors",
  notes: "/app/notes",
  tasks: "/app/tasks",
  console: "/app/console",
  traces: "/app/traces",
  extraction: "/app/extraction",
  ontology: "/app/ontology",
  sessionCache: "/app/session-cache",
  actions: "/app/actions",
  mesh: "/app/mesh",
  batchSearch: "/app/batch-search",
  graphGlobal: "/app/graph/global",
  graphEgo: "/app/graph/ego",
  graphTimeline: "/app/graph/timeline",
  diagnostics: "/app/diagnostics",
} as const;

export const STORAGE_KEYS = {
  token: "brain_sentry_token",
  user: "brain_sentry_user",
  tenantId: "tenant_id",
  language: "brainsentry.lang",
} as const;

export const E2E_LANGUAGE = "pt-BR";
