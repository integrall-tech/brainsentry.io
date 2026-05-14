# CLAUDE.md — anotações arquiteturais para Claude Code

> Este arquivo complementa `AGENTS.md`. Aqui você encontra os "key files" do
> projeto com explicação de **por que cada um existe**, **invariantes
> escondidas** e **bugs históricos** que cravaram aprendizados no código.
>
> Leia `AGENTS.md` primeiro para o contexto geral.

---

## Camadas e fluxo de dados

```
┌──────────────────────────────────────────────────────────────┐
│ Frontend (React)  ──REST──▶  Backend Go  ──Cypher──▶ FalkorDB│
│ TUI (Bubble Tea)  ──REST──▶  Backend Go  ──pgvector─▶ Postgres│
│ Agente IA         ──MCP───▶  Backend Go  ──Redis───▶ Cache    │
│                              │                                │
│                              └──HTTP──▶ OpenRouter / Anthropic│
└──────────────────────────────────────────────────────────────┘
```

- **System of record (canonical)**: PostgreSQL — tabelas `memories`,
  `users`, `tenants`, `audit_logs`, `notes`, etc.
- **Derivado (cache rebuild-able)**: FalkorDB (knowledge graph), Redis
  (embeddings, ratelimit), `communities` (Louvain), embeddings reindexáveis.
- **Quando você muda uma tabela canonical**: pense em migração + reconciler.
  Quando muda um cache: documente como reconstruir.

---

## Backend Go — key files anotados

### Camada HTTP

`brain-sentry-go/cmd/server/main.go`
> Entry point. Cabeamento de TODOS os services e handlers. Quando você
> adiciona um service novo, é aqui que ele entra (composition root).
> **Atenção**: ordem de inicialização importa quando há dependência mútua.

`brain-sentry-go/internal/handler/`
> Cada arquivo expõe um conjunto de rotas. Padrão: handler é fino, delega
> tudo pra `internal/service`. Não coloque lógica de negócio em handler.

`brain-sentry-go/internal/middleware/auth.go`
> JWT + tenant resolution. **Toda rota autenticada precisa do middleware
> `Tenant`** depois do `Auth`, senão `tenant.FromContext(ctx)` retorna
> string vazia e queries trazem dados de outros tenants (vulnerabilidade!).

`brain-sentry-go/internal/middleware/cors.go`
> Allowlist em `config.yaml`. Se localhost:5xxx não funciona, é aqui.

### Services críticos

`brain-sentry-go/internal/service/interception.go`
> **Caminho quente**: prompt do agente entra, contexto relevante sai.
> Pipeline: quickCheck → deepAnalysis (LLM) → vector search → GraphRAG
> enrichment → fallback text search → filter expired/superseded →
> filter by importance → format with token budget. **Toda memória
> injetada passa por `security.FrameMemory()`** desde o sanitizer P1.

`brain-sentry-go/internal/security/injection.go`
> Sanitizer de prompt-injection. 14 patterns regex + structural framing
> `<memory id="..." source="...">`. Único point-of-truth pro pattern set
> — qualquer caminho que injete conteúdo terceiro **precisa** usar
> `Sanitize()` ou `FrameMemory()`. Telemetria via `slog` em
> `interception.go:formatContextWithBudget`.

`brain-sentry-go/internal/service/auto_forget.go`
> 3 mecanismos: TTL expiry, contradição via Jaccard >0.9 (substitui
> memory antiga marcando `superseded_by`), low-value cleanup (helpful=0
> e injection_count=0 e age>30d). **Cuidado**: o cleanup é destrutivo;
> roda async. Se um teste flakea aqui é race com o scheduler.

`brain-sentry-go/internal/service/cascading_staleness.go`
> Quando `superseded_by` é setado, propaga "stale" via BFS no grafo
> com decay per-hop. Isso é o que mantém o ego-graph consistente.

`brain-sentry-go/internal/service/spreading_activation.go`
> BFS com decay exponencial. **Não confundir com cascading staleness**:
> spreading activation é leitura (rank), staleness é escrita (mark).

`brain-sentry-go/internal/service/llm_provider.go` + `*_provider.go`
> Interface `LLMProvider` + impls (`openrouter`, `anthropic`, `gemini`).
> Wrapped em `FallbackChainProvider` + `CircuitBreaker`. Embedding
> service tem provider próprio (OpenAI por padrão).

`brain-sentry-go/internal/service/pii.go`
> 8 tipos de PII detectados; modo MASK (default) substitui por
> `[EMAIL]`, `[SSN]`, etc. Modo STRIP (não default ainda) remove
> totalmente. Chamado em `interception.go` ANTES da injeção no LLM.

`brain-sentry-go/internal/service/query_router.go`
> Regex-based classifier (sem LLM) que escolhe entre 6+ search types.
> Latência ~1ms vs ~500ms da `AnalyzeRelevance` LLM call.
> **Order matters**: o primeiro pattern a matchar ganha.

### Repositories

`brain-sentry-go/internal/repository/postgres/memory.go`
> `MemoryRepository`. Métodos importantes: `FullTextSearch`,
> `FindByRecordedRange`, `IncrementInjectionCount`. Usa pgvector pra
> embeddings (cosine). Schema fica em `cmd/server/migrations/`.

`brain-sentry-go/internal/repository/graph/`
> `MemoryGraphRepository` (CRUD em FalkorDB) + `GraphRAGRepository`
> (multi-hop search, EnrichContext). Cypher fica inline (não temos
> migrations de schema FalkorDB ainda — é cache).

### Domain

`brain-sentry-go/internal/domain/memory.go`
> Modelo `Memory`. Campos relevantes:
> - `Importance` (CRITICAL/IMPORTANT/NORMAL/LOW)
> - `Category` (KNOWLEDGE/DECISION/PATTERN/...)
> - `RecordedAt`, `ValidFrom`, `ValidTo`, `SupersededBy` (bi-temporal)
> - `HelpfulCount`, `NotHelpfulCount`, `InjectionCount` (feedback)
> - `BelongsToSets` (NodeSets — agrupamento ad-hoc)
> - `FeedbackWeight` (0-1, blend em scoring)

---

## Frontend React — key files anotados

`brain-sentry-frontend/src/components/layout/AdminLayout.tsx`
> Sidebar agrupada (7 grupos + Dashboard) com modo rail colapsável.
> Persistência em localStorage: `bs_visited`, `bs_nav_expanded`,
> `bs_sidebar_collapsed`. **Cuidado** ao mudar selectors usados pelos
> testes E2E (`data-testid="sidebar"`, `data-collapsed`,
> `data-testid="sidebar-toggle"`).

`brain-sentry-frontend/src/lib/help/helpContent.ts`
> Conteúdo dos drawers de ajuda — 31 telas × 2 idiomas. **Foco em
> negócio, sem jargão técnico**. Quando criar tela nova, adicione aqui
> ANTES de mergear (E2E `screen-help.spec.ts` valida).

`brain-sentry-frontend/src/components/ui/ScreenHelp.tsx`
> Drawer da direita. Fecha via X, overlay click ou ESC. Selectors
> `data-testid="screen-help-drawer"`, `screen-help-overlay`,
> `screen-help-trigger`.

`brain-sentry-frontend/src/lib/api/client.ts`
> Source of truth de tipos da API. Quando o backend mudar a forma de
> uma resposta, **edite aqui primeiro** e o TS vai apontar todos os
> consumidores quebrados.

`brain-sentry-frontend/src/pages/GraphGlobalPage.tsx`
> Visualização de grafo global (react-force-graph-2d). **Importante**:
> precisa do `import "d3-transition"` no topo, senão `selection.interrupt
> is not a function` em runtime (bug histórico — d3-zoom chama
> `.interrupt()` que só é attached pelo d3-transition).

`brain-sentry-frontend/src/pages/GraphTimelinePage.tsx`
> SVG custom (não force-graph). X = recorded_at, lanes = category,
> setas vermelhas = SUPERSEDES. `<marker>` def no `<defs>` do SVG.

---

## Bugs históricos (lembre-se)

### Strict-mode selector violation com `truncate` na sidebar
- **Sintoma**: E2E `userEmail` quebrava com "matched 2 elements".
- **Causa**: classe `truncate` num subtitle do header colidia com o seletor
  `aside .text-xs.text-muted-foreground.truncate`.
- **Fix**: não usar `truncate` no subtitle do logo. Veja
  `AdminLayout.tsx` — header do logo é minimalista.

### Toast race em `mesh-pages.spec.ts`
- **Sintoma**: assertion `toBeVisible({ timeout: 10000 })` falhava sem motivo.
- **Causa**: toast auto-dismiss em 5s, e a assertion pegava o gap.
- **Fix**: `waitForResponse('/v1/mesh/peers')` antes do assert. Nunca
  espere mais que o auto-dismiss do toast (5s) sem âncora de rede.

### Chevron click ambíguo no `cognee-pages.spec.ts`
- **Sintoma**: depois que sidebar virou agrupada, `button:has(svg.lucide-chevron-down)`
  pegava o chevron do grupo na sidebar antes do chevron de "expand row" no main.
- **Fix**: scope para `main button:has(...)`.

### CORS bloqueando login da porta 5175
- **Sintoma**: login retornava 401 silenciosamente do navegador na 5175.
- **Causa**: `config.yaml` `allowed_origins` não tinha 5174/5175.
- **Fix**: vite default é 5173, mas se a porta cai em 5174/5175 também
  precisa estar no allowlist.

### react-force-graph crashava sem d3-transition
- **Sintoma**: ErrorBoundary com `TypeError: selection4.interrupt is not a function`.
- **Causa**: d3-zoom interno chama `.interrupt()` em selections;
  `interrupt` só é attached pelo `d3-transition`.
- **Fix**: `npm install d3-transition` + `import "d3-transition"` no
  topo das pages que usam force-graph.

---

## Observabilidade

- **Logs**: `slog` (JSON em produção, text em dev). Telemetria de
  sanitization fica em `interception.go` com `slog.Warn(...)`.
- **Métricas**: Prometheus `/metrics`. Counters por handler, latency
  histograms.
- **Audit**: tudo que altera memória passa por `audit.go` async.

---

## Antes de fechar PR

1. `go build ./... && go test ./... -short -count=1` — verde.
2. `cd brain-sentry-frontend && npm run build && npx playwright test`
   — verde.
3. Se mexeu em handler: rota documentada em `swagger.go`.
4. Se mexeu em service: teste unitário cobrindo o caminho feliz +
   pelo menos 1 edge case.
5. Se mexeu em UI: entrada nova/atualizada em `helpContent.ts` E spec
   E2E em `e2e/tests/`.
6. **Sem `--no-verify`**. Se hook quebrou, conserte a causa.

---

## Onde estão as coisas

- Specs OpenAPI/swagger: `brain-sentry-go/docs/swagger.go`
- Migrations SQL: `brain-sentry-go/cmd/server/migrations/`
- Seeds (demo user etc.): `brain-sentry-go/cmd/cli/`
- Config: `brain-sentry-go/config.yaml`
- E2E helpers: `brain-sentry-frontend/e2e/{fixtures,helpers,pages}/`
- i18n strings: `brain-sentry-frontend/src/i18n/locales/{pt-BR,en}.json`
- Plan/decisão arquitetural história: `documents/`
