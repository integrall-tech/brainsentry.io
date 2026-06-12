# Auditoria de Prontidão — Homologação / Produção

> Data: 2026-06-12 · Branch auditada: `feat/sidebar-rail-tooltips`
> Escopo: brain-sentry-go, brain-sentry-frontend, infra/deploy, testes E2E, testes Go.
> Todos os achados CRÍTICOS foram verificados manualmente na fonte (file:line).

## Veredito

**Não está pronto para homologação ainda.** O codebase é sólido (build/vet limpos,
sanitizer de injection 100% testado, migrations com down, graceful shutdown,
multi-stage Dockerfiles), mas há **6 bloqueadores críticos** — 3 de segurança
ativa (segredo vazado, bypass de tenant, endpoints sem RBAC) — e a cobertura E2E
do admin tem 4 telas com zero testes.

---

## CRÍTICOS (bloqueiam homologação)

### C1. API key real do OpenRouter commitada no git
`brain-sentry-backend/src/main/resources/application.yml:88`
```
api-key: ${BRAINSENTRY_AI_AGENTIC_MODEL_API_KEY:sk-or-v1-60ee547f...}
```
O *default* do placeholder é uma chave real, presente no histórico do git em
múltiplos commits. Também na linha 53/76: senha do FalkorDB `Relevant727204`.
**Ação: revogar a chave OpenRouter e trocar a senha do FalkorDB AGORA**,
independente de qualquer outra coisa. Depois limpar os defaults do YAML
(deixar `${VAR:}` sem fallback) e considerar reescrita de histórico ou
tratar o repositório como comprometido para esses segredos.

### C2. Bypass de isolamento de tenant via header `X-Tenant-ID`
`brain-sentry-go/internal/middleware/tenant.go:12-40`
A prioridade de resolução é: **header > query param > JWT claim > default**.
Um usuário autenticado do tenant A envia `X-Tenant-ID: B` e o contexto passa a
ser o tenant B — vulnerabilidade cross-tenant direta, contradiz a regra do
próprio CLAUDE.md. **Ação: inverter a prioridade — JWT claim é a fonte de
verdade; header/query só podem ser aceitos se o claim autorizar (ex.: role
admin global) ou removidos por completo.** Adicionar teste de regressão.

### C3. Endpoints administrativos sem `RequireRole`
`brain-sentry-go/cmd/server/main.go:1104` — `POST /v1/pii/scan`
`brain-sentry-go/cmd/server/main.go:1110` — `POST /v1/semantic/consolidate`
Ambos no mesmo handler/área admin, mas fora do grupo `RequireRole(RoleAdmin)`
(compare com `/v1/auto-forget` na linha 1107, que está protegido).
**Ação: mover para o grupo admin ou aplicar `r.With(middleware.RequireRole(...))`.**

### C4. Tenant ID demo hardcoded como fallback no frontend (9 ocorrências)
`a9f814d2-4dae-41f3-851b-8aa3d4706561` em:
- `src/contexts/AuthContext.tsx:31`
- `src/hooks/index.ts:27`
- `src/lib/api/client.ts:404`
- `src/pages/UsersPage.tsx:67`, `TenantsPage.tsx:66`, `AuditPage.tsx:71,101`,
  `ConfigurationPage.tsx:614`, `DashboardPage.tsx:58`
Usuário sem `tenant_id` no localStorage cai silenciosamente no tenant demo
compartilhado. Combinado com C2 (header tem prioridade máxima no backend),
isso vira vazamento real de dados. **Ação: fail-closed — sem tenantId, deslogar
e redirecionar para login. Centralizar resolução de tenant num único helper.**

### C5. Credenciais demo e segredos default em código/config
- `brain-sentry-go/internal/service/auth.go:111-112` — `demoPassword = "demo123"`
  + demoTenantID fixo; rota `/v1/auth/demo` sem gate de ambiente.
- `brain-sentry-go/config.yaml:11,28` — senha do Postgres e JWT secret default.
- `brain-sentry-frontend/src/pages/LoginPage.tsx:53-64` — botão demo com
  `demo@example.com/demo123` hardcoded.
- `docker-compose.production.yml` / `brain-sentry-go/docker-compose.yml` —
  senhas default (`brainsentry`, `change-me-in-production-please`).
**Ação: demo login atrás de flag de config (`auth.demo_enabled: false` em
prod) e botão demo atrás de `import.meta.env.VITE_DEMO_LOGIN`. Server deve
RECUSAR boot em modo produção com JWT secret default. Todos os segredos via
env vars / secrets do orquestrador.**

### C6. Postgres com `sslmode=disable` hardcoded
`brain-sentry-go/internal/config/config.go:47` — DSN com `sslmode=disable`
fixo no fmt.Sprintf. **Ação: tornar `sslmode` configurável, default
`require` quando `env=production`.**

---

## ALTOS (corrigir antes de produção; homologação tolerável com ressalva)

| # | Achado | Local | Ação |
|---|--------|-------|------|
| A1 | Mudanças do branch atual não commitadas (`tooltip.tsx` untracked é dependência do `AdminLayout.tsx` modificado — build quebra sem ele) | `src/components/ui/tooltip.tsx`, `AdminLayout.tsx` | Commitar/mergear o branch `feat/sidebar-rail-tooltips` antes de qualquer build de homologação |
| A2 | `cascading_staleness.go` (154 linhas, BFS com mutação de grafo) sem NENHUM teste | `internal/service/cascading_staleness.go` | Criar teste unitário: propagação, ciclos, maxDepth |
| A3 | Cobertura de handlers: 10/63 (~16%) | `internal/handler/` | Priorizar memory, auth, webhook, tenant |
| A4 | Race no webhook service entre `Unregister` e `Emit` (índice `byTenant`) + delivery sem context timeout | `internal/service/webhook.go:106-115,231` | RWMutex consistente + `context.WithTimeout` no Do |
| A5 | Imagens `falkordb:latest` e `adminer:latest` em produção | `docker-compose.production.yml:30` e outros | Pinnar tags semânticas |
| A6 | Hibernate `ddl-auto: update` no backend Java legado (se ainda deployado) | `brain-sentry-backend/.../application.yml:34` | `validate` em prod — ou confirmar que o Java está fora do deploy e removê-lo do compose de produção |
| A7 | Dashboard e outras páginas sem estado de erro visual (spinner eterno em falha de API) | `DashboardPage.tsx:66-82`, `GraphEgoPage.tsx`, `RelationshipsPage.tsx` | Estado de erro + retry |
| A8 | CORS de produção inclui localhost por default | `docker-compose.production.yml:71` | Allowlist só de domínios reais via env |
| A9 | Backend exposto direto na porta 8080 no compose de produção | `docker-compose.production.yml:75` | Só via reverse proxy |

## MÉDIOS

- M1. Migrations Go não auto-aplicadas no boot (operador roda psql manualmente) — documentar runbook ou implementar auto-migrate com lock (`Dockerfile:49-51`).
- M2. `/metrics` (Go) e `/actuator/metrics|prometheus` (Java) públicos — restringir por IP no proxy ou autenticar.
- M3. Sem backup automatizado do Postgres no compose de produção — adicionar job de `pg_dump` + retenção (swarm-deploy.md menciona `/app/backups` mas o compose não implementa).
- M4. `Forget by set` retorna "not yet supported" em runtime (`internal/service/semantic_api.go:336`) — implementar ou retornar 501 documentado no swagger.
- M5. `console.log` de email + `alert()` na landing (`src/landing/components/cta/CTASection.tsx:12-14`).
- M6. URLs `localhost:8080` como fallback de `VITE_API_URL` espalhadas em 9+ arquivos — centralizar em `client.ts`; build de produção deve falhar se `VITE_API_URL` ausente.
- M7. `SPRING_PROFILES_ACTIVE=prod` não forçado no compose (log DEBUG default no Java).
- M8. Erro engolido em `internal/handler/eval.go:29` (best-effort aceitável, mas logar).
- M9. Healthcheck `start_period: 60s` pode ser curto para boot com migrations.

## BAIXOS

- B1. Versão `v1.0.0` hardcoded no `AdminLayout.tsx:523` — interpolar do package.json.
- B2. JWT em localStorage (padrão SPA aceitável; documentar trade-off).
- B3. `lazy.MustGet()` panica — auditar que só é usado em recursos mandatórios.
- B4. Goroutines background com `context.Background()` sem timeout em `memory.go`.
- B5. Strings PT hardcoded pontuais fora do i18n (ex.: `PoliciesPage.tsx` placeholder).

## Falsos positivos descartados na verificação

- `.env.local` do frontend **não** está commitado (só `brain-sentry-explorer/.env.example` está no git).
- helpContent.ts está **completo**: 33 páginas × 2 idiomas presentes.
- Sem SQL/Cypher injection encontrada (queries parametrizadas + `EscapeCypher`).

---

## Cobertura E2E do Admin — resposta direta: NÃO cobre tudo

34 páginas, 13 specs, 3 browsers (chromium, firefox, mobile-chrome), pt-BR.
Mocks centralizados em `admin-mocks.ts` (bom determinismo). Testes contra API
real existem mas ficam atrás de `E2E_REAL_API=1` (skipados por default).

| Profundidade | Qtd | Páginas |
|---|---|---|
| **Zero testes** | 4 | Policies, Events, Reasoning, Provenance |
| **Só smoke** | 8 | Dashboard, Audit, Analytics, Profile, Connectors, Notes, Tasks, Decisions |
| **Interação** | 20 | Memories, Search, Users, Tenants, Configuration, Console, Traces, Extraction, Ontology, SessionCache, Actions, Mesh, BatchSearch, Graphs (3), Diagnostics, Models, Playground, Timeline, Relationships |
| **CRUD/Auth completos** | 2 | Memory Integrity (real API), Login |

**Fluxos críticos sem cobertura:**
1. **Logout** — nunca testado (login sim).
2. **RBAC** — nenhum teste de ADMIN vs USER, acesso negado a rota protegida.
3. **Multi-tenancy** — nenhum teste de isolamento/troca de tenant (relevante dado C2/C4!).
4. **UPDATE/DELETE de memórias na UI** — só CREATE/READ via mocks.
5. **Users/Tenants** — sem update, delete, role assignment.
6. **Erros de rede/validação** — nenhum teste de falha de API renderizando estado de erro.

---

## Plano de correção

### Fase 0 — Emergência (hoje, ~meio dia)
1. **Revogar chave OpenRouter `sk-or-v1-60ee...` e trocar senha FalkorDB** (C1).
2. Remover defaults sensíveis do `application.yml` legado e do `config.yaml`.
3. Commitar/mergear `feat/sidebar-rail-tooltips` (A1).

### Fase 1 — Segurança backend (1–2 dias) → desbloqueia homologação
4. Corrigir prioridade do `TenantExtractor`: JWT primeiro; header só com autorização explícita (C2) + teste de regressão cross-tenant.
5. `RequireRole(RoleAdmin)` em `/v1/pii/scan` e `/v1/semantic/consolidate` (C3).
6. Demo login atrás de flag (`auth.demo_enabled`) + boot recusa JWT secret default em produção (C5).
7. `sslmode` configurável, `require` em prod (C6).
8. Fix race + timeout no webhook service (A4).

### Fase 2 — Frontend fail-closed (1 dia)
9. Remover os 9 fallbacks de tenant hardcoded — helper único, fail-closed (C4).
10. Centralizar `API_URL`/`WS_URL` em `client.ts`; build falha sem `VITE_API_URL` em prod (M6).
11. Estados de erro visuais no Dashboard e páginas listadas em A7.
12. Limpar `console.log`/`alert` da landing (M5); botão demo atrás de env (C5).

### Fase 3 — Infra de homologação (1–2 dias)
13. Compose de homologação: secrets via env/secrets manager, tags pinadas (A5), CORS sem localhost (A8), backend só atrás de proxy (A9), `SPRING_PROFILES_ACTIVE=prod` (M7) — ou remover o Java legado do deploy (A6).
14. Runbook de migrations (ou auto-migrate com advisory lock) (M1).
15. Backup do Postgres com retenção (M3). Proteger `/metrics` (M2).

### Fase 4 — Testes (paralelo às fases 1–3, ~3 dias)
16. `cascading_staleness_test.go` (A2).
17. Testes de handler: memory, auth, webhook, tenant (A3) — incluindo teste do C2/C3 corrigidos.
18. E2E novos: Policies, Events, Reasoning, Provenance (4 specs); logout; RBAC (admin vs user); UPDATE/DELETE de memória; erro de API renderizando estado de erro.
19. Rodar suite `E2E_REAL_API=1` contra o ambiente de homologação como gate de release.

### Fase 5 — Gate de produção
20. Re-rodar checklist do CLAUDE.md (build + test + playwright verdes).
21. Pentest rápido focado em tenant isolation (validar C2/C4 corrigidos).
22. Smoke em homologação por ≥1 semana antes de produção.

**Estimativa total: ~7–9 dias úteis de trabalho até "pronto para homologação com gate de produção definido".**
