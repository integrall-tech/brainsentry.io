# Baseline — Brain Sentry (brainsentry.io) — 2026-07-02

> Auditoria somente-leitura para o roadmap do VendaX Sales Copilot (IntegrAllTech).
> Método: leitura de código + execução real de build/testes nesta máquina (macOS, sem Docker disponível).
> Toda afirmação cita evidência; o que não pôde ser exercitado está marcado **NÃO VERIFICADO**.

## 1. Identificação

- **Nome**: Brain Sentry — "Agent Memory System for Developers" (`README.md:1-3`). Plataforma de memória cognitiva persistente para agentes de IA: memória bi-temporal, knowledge graph, interceptação de prompt com injeção de contexto, MCP server (`AGENTS.md:8-17`).
- **Propósito em 2 frases**: dá memória de longo prazo a agentes de IA (grava, versiona, decai, supersede e recupera memórias com busca híbrida vetor+grafo+texto). Expõe isso via REST, MCP (JSON-RPC 2.0 + SSE), admin UI React, TUI e CLI.
- **Stack e versões**:
  - Backend principal: **Go 1.25** (`brain-sentry-go/go.mod:3`), chi v5, pgx v5, go-redis v9, JWT v5, cobra, Bubble Tea (TUI).
  - Frontend: **React 19 + Vite 6 + TypeScript 5.7** (`brain-sentry-frontend/package.json`), Radix UI, i18next pt-BR/en, react-force-graph/cytoscape/echarts.
  - Dados: **PostgreSQL 16 + pgvector** (canonical), **FalkorDB** (grafo, cache reconstruível), **Redis** (cache/ratelimit/tasks).
  - LLM: OpenRouter + Anthropic + Gemini nativos em fallback chain com circuit breaker (`cmd/server/main.go:258-316`).
  - Backend legado: Java/Spring Boot 4 em `brain-sentry-backend/` — **morto** ("Não tocar", `AGENTS.md:24`; fora do CI e do compose de produção).
  - Auxiliares: `brain-sentry-explorer/` (TUI Ink/Node de validação + benchmark), `plugins/claude-code/` (plugin MCP).
- **Governança/spec**: não há `openspec/` nem `adr/` (busca vazia). Specs históricas em `documents/` (era Java, congeladas) + `EXECUTION_PLAN.md` (desatualizado, ver §7). Docs vivos: `AGENTS.md`, `CLAUDE.md`, `docs/deploy/swarm-deploy.md`.
- **Atividade**: 48 commits, de 2026-01-19 a **2026-06-28** (último commit — 4 dias atrás). Cadência contínua (jan, mar, abr, mai, jun). **1 contribuidor** (Edson Martins, sob 2 identidades — `git shortlog -sn HEAD`). Projeto ATIVO.

## 2. Saúde de build e testes

Comandos executados nesta auditoria (2026-07-02):

| Comando | Resultado real |
|---|---|
| `cd brain-sentry-go && go build ./...` | **OK** (exit 0) |
| `go test ./... -short -count=1` | **OK — 22 pacotes, 1281 testes PASS / 0 FAIL / 1 SKIP** (contagem via `-v` + grep `--- PASS/FAIL/SKIP`) |
| Testes de integração Go (testcontainers) | **NÃO EXECUTADOS** — `internal/repository/postgres/integration_test.go` e `internal/service/auto_forget_integration_test.go` exigem Docker, indisponível nesta máquina |
| `cd brain-sentry-frontend && npm ci` | **FALHA** — não existe `package-lock.json` no frontend (build não-reproduzível) |
| `npm install && npm run build` (tsc + vite) | **OK** — 3496 módulos, built in 3.02s; aviso: bundle único de 2.37 MB (724 kB gzip), sem code-splitting |
| `npx vitest run` (script `npm test`) | **QUEBRADO** — vitest sem config coleta os specs Playwright de `e2e/` e falha ("17 failed, no tests"); não existe nenhum teste unitário em `src/` (find vazio) |
| `npx playwright test --project=chromium` | **OK — 74 testes passando em 40.7s** (17 specs, modo mock de API; modo `test:e2e:real` existe mas exige backend vivo — NÃO EXECUTADO) |
| Cobertura | Não configurada/reportada em nenhum dos dois lados |

## 3. Capabilities e maturidade

Escala 0-5 conforme instrução. "Spec/Código": (a) só spec, (b) código parcial, (c) código funcional.

| Capability | Maturidade (0-5) | Spec/Código | Evidência |
|---|---|---|---|
| CRUD + busca híbrida de memórias (pgvector, BM25, versioning, rollback, feedback, dedup SimHash) | 4 | (c) funcional | `internal/repository/postgres/memory.go`, `internal/service/`; 66 arquivos de teste em `internal/service`; homologado internamente (PRs #1, #13) |
| Interceptação de prompt (quickCheck → LLM → vetor → GraphRAG → fallback texto → budget de tokens → PII mask → framing) | 4 | (c) funcional | `internal/service/interception.go:87-447` — pipeline completo, sem etapas stub; requer chave LLM |
| Bi-temporal (as-of, changed-since, supersessão + cascading staleness) | 4 | (c) funcional | `internal/service/cascading_staleness.go`, rotas `/v1/memories/as-of`, `/changed-since` (PR #6) |
| Trust & provenance (score explicável, PROV-O export) + conflitos (detect/scan/resolve) | 4 | (c) funcional | `pkg/trust/`, `/v1/memories/{id}/trust`, `/v1/conflicts/*` (PRs #5, #8) |
| Ingestão de documentos (txt/md/csv/json/docx → memórias) | 4 | (c) funcional | `POST /v1/memories/upload` (PR #7) |
| MCP server (7 tools, 3 resources, 6 prompts; JSON-RPC 2.0 + SSE + batch, sob JWT) | 4 | (c) funcional | `internal/mcp/{server,tools,resources,prompts}.go`; rotas `cmd/server/main.go:1038-1041`. **NÃO VERIFICADO em runtime** (sem infra nesta máquina) |
| Grafo de conhecimento FalkorDB (GraphRAG multi-hop, entity graph, comunidades Louvain, NL→Cypher, spreading activation) | 3 | (c) funcional com ressalva | Cypher real de escrita/leitura em `internal/repository/graph/` (`MERGE (m:Memory...)`, `MATCH path=...*1..N`). **Ressalva**: nós Memory só entram no grafo via rebuild (`internal/rebuild/targets.go:87`), não no create em tempo real; escrita em tempo real só para entidades. Fallbacks nil-safe se FalkorDB fora |
| Modelo cognitivo (auto-forget TTL/contradição/low-value, reflexão, consolidação, decay) | 3 | (c) funcional | `internal/service/auto_forget.go`, `spreading_activation.go`; cleanup destrutivo async; rotas condicionais a LLM |
| Governança (decisions, policies + enforcement, events, raciocínio abdutivo) | 3 | (c) funcional | migration `000008_decisions_policies_events`, `internal/handler/{decision,policy,event,reasoning}.go`; sem doc/spec formal |
| Frontend admin (34 páginas, i18n pt-BR/en, help drawer, grafos) | 4 | (c) funcional | `src/pages/` (34 arquivos); build OK; **74 E2E Playwright passando** (mock); dev server ativo na porta 5173 durante a auditoria |
| Multi-tenancy + RBAC + fail-closed config | 4 | (c) funcional com riscos | Ver §6 — coluna `tenant_id` consistente no Postgres; riscos: default tenant fallback, grafo único FalkorDB |
| Segurança de conteúdo (sanitizer 14 patterns + framing, PII 8 tipos, trust boundaries) | 4 | (c) funcional | `internal/security/injection.go`, `internal/service/pii.go`, chamados em `interception.go:223-227,413,440` |
| Webhooks | 3 | (c) funcional | `internal/handler/webhook.go`, migration `000005_add_webhooks`. NÃO VERIFICADO em runtime |
| Mesh P2P / Actions+leases / Agent traces | 3 | (b) parcial | Lógica completa mas **estado in-memory** (`internal/service/{actions,mesh_sync,agent_trace}.go` — map+mutex; main.go:419 "in-memory for now"); perde tudo no restart |
| Conectores externos (GitHub/Notion/Drive/WebCrawler) | 2 | (b) parcial / semi-stub | HTTP real em `internal/service/connector.go:439-699`, mas **`ConnectorRegistry` nunca é populado** (main.go:247; nenhum `Register()` fora de testes) — `GET /v1/connectors` retorna vazio em runtime; ingestão "simplified" (JSON bruto como texto) |
| CLI (`brainsentry` cobra, 12 comandos) | 2 | (b) parcial | **Verificado por execução**: `init --embedded` funciona; `add`/`search`/`list` retornam "memory service not configured" — `cmd/cli/main.go` nunca injeta services no `App` (só os testes injetam mocks) |
| Store plugável / modo embedded (JSON, zero-deps) | 3 | (c) funcional no server | `internal/store/` + `/v1/store/memories`; mas boot do server ainda exige Postgres (verificado: `go run ./cmd/server` → exit 1 sem Postgres) e o CLI embedded não faz CRUD (linha acima) |
| TUI (Bubble Tea, 9 views) | 3 | (c) funcional (não testado) | `cmd/tui/` — cliente REST completo; **zero testes** nos subpacotes |
| Explorer + benchmark de retrieval reprodutível | 3 | (c) funcional | `brain-sentry-explorer/` — `"benchmark": "tsx src/cli.tsx --benchmark"`, cenários em `src/scenarios/` (inclui `sales-corpus.ts`); exige backend vivo — NÃO VERIFICADO em runtime |
| Plugin Claude Code (6 skills + MCP) | 2 | (b) parcial, com drift | `plugins/claude-code/plugin.json` aponta porta 8082 e rota `/v1/mcp` sem sufixo — servidor real é 8081 + context path `/api` + `/v1/mcp/{message,sse,batch}`; sem commits desde 2026-04-23 |
| Backend Java/Spring | 0 (morto) | (b) abandonado | `brain-sentry-backend/` — 118 arquivos .java fora do CI e do compose de produção; `AGENTS.md:24` "Não tocar" |
| Endpoints `/api/v1/integration/execution/{start,end}` | 2 | (b) stub declarado | inline em `cmd/server/main.go:1251-1263` — resposta fixa `{"status":"ok"}` |

## 4. Superfícies expostas (o que dá para consumir hoje)

Tudo abaixo existe em código funcional e compilado; **status runtime NÃO VERIFICADO nesta máquina** (sem Docker → sem Postgres/FalkorDB; server faz fail-fast sem Postgres — verificado por execução).

**REST (chi, sob `context_path: /api`, porta 8081; JWT obrigatório exceto públicos):**
- Memória: `POST/GET/PUT/DELETE /v1/memories`, `/search`, `/plan-search`, `/batch-search`, `/upload`, `/as-of`, `/changed-since`, `/{id}/versions|rollback|feedback|trust|flag|review`, `/activate`
- Agente: `POST /v1/intercept` (injeção de contexto), Semantic API `POST /v1/remember|recall|improve|forget`
- Grafo: `/v1/graph/{global,ego,timeline,communities,nl-query}`, `/v1/entity-graph/*`, `/v1/relationships` (11 rotas) — condicionais a FalkorDB up
- Governança: `/v1/decisions` (8 rotas), `/v1/policies`, `/v1/events`, `/v1/reasoning/abduce`, `/v1/export/provenance`, `/v1/audit`, `/v1/conflicts/*`
- Operação: `/v1/auth/*` (login/refresh/demo/SSO), `/v1/users`, `/v1/tenants`, `/v1/webhooks`, `/v1/store/memories`, `/health`, `/version`, `/metrics` (Prometheus), `/swagger.json`
- Públicos sem auth: `/health`, `/version`, `/metrics`, `/swagger.json`, `/v1/auth/*`, `/v1/diagnostics`, `/v1/models` (ver risco §6)

**MCP (JSON-RPC 2.0)** — `POST /v1/mcp/message`, `/v1/mcp/sse` (event-stream), `/v1/mcp/batch`, sob JWT:
- Tools (7): `intercept_prompt`, `create_memory`, `search_memories`, `get_memory`, `list_memories`, `update_memory`, `delete_memory` (`internal/mcp/tools.go:17-113`)
- Resources (3): `brainsentry://memories|notes|hindsight`; Prompts (6): `capture_pattern`, `extract_learning`, `summarize_discussion`, `context_builder`, `agent_context`, `memory_summary`

**Eventos/filas**: nenhum broker externo (sem Kafka/Rabbit). Webhooks outbound (`/v1/webhooks`, tabela própria com deliveries) são o único push. Tasks assíncronas via Redis (`/v1/tasks`, condicional a Redis).

**SDKs**: não há SDK publicado. Clientes existentes: TUI Go (`internal/client`), explorer Node (axios), plugin Claude Code (com drift de porta/rota).

## 5. Integrações consumidas

| Dependência | Uso | Por tenant? |
|---|---|---|
| OpenRouter | LLM default + embeddings (`openai/text-embedding-3-small`, 384d) | **NÃO — global por servidor** (`config.go:294`, env `BRAINSENTRY_AI_AGENTIC_MODEL_API_KEY`); todos os tenants compartilham chave e quota |
| Anthropic / Gemini (nativos) | Fallback chain (Anthropic > Gemini > OpenRouter) + circuit breaker | **NÃO — global**, opt-in via `config.yaml` (sem env override para essas chaves) |
| PostgreSQL 16 + pgvector | System of record | infra única, isolamento por coluna |
| FalkorDB | Knowledge graph (cache reconstruível) | grafo único `brainsentry`, isolamento por propriedade `tenantId` no Cypher |
| Redis | Cache de embeddings, ratelimit, task scheduler | global; features degradam se ausente |
| GitHub / Notion / Google Drive / WebCrawler | Conectores de ingestão | **semi-stub** — registry nunca populado (§3); credenciais nem persistidas |
| Outros produtos IntegrAllTech | Nenhuma integração direta no código; `/api/v1/integration/*` é stub e `SQUADX_SERVICE_SECRET` (`internal/middleware/service_auth.go`) sugere integração planejada com SquadX — middleware **não usado** em nenhuma rota | — |

## 6. Multi-tenancy, segurança e dados

**Isolamento**: shared DB/shared schema, coluna `tenant_id` em todas as tabelas de negócio; filtragem consistente verificada (`memory.go:122,191,221,261,348` etc.). JWT claim é a fonte de verdade do tenant; header `X-Tenant-ID` só pode divergir se role ADMIN (`internal/middleware/tenant.go:22-39`) — o bypass C2 da auditoria interna de 2026-06-12 foi corrigido no hardening P0 (commit 536265e).

**Riscos abertos**:
1. `tenant.FromContext()` retorna **tenant default hard-coded** quando o contexto não tem tenant (`pkg/tenant/context.go:11,20-25`) — fail-open para jobs async/código novo (CLAUDE.md diz que retorna string vazia; está desatualizado).
2. FalkorDB: grafo único, isolamento apenas por filtro `tenantId` em Cypher **string-interpolado** com `EscapeCypher` (`client.go:234`) — sem enforcement estrutural; um filtro esquecido = vazamento cross-tenant.
3. `/v1/diagnostics` e `/v1/models` públicos sem auth (match por prefixo em `main.go:832-842`).
4. Demo login cria usuário **ADMIN** com credenciais fixas (`internal/service/auth.go:110-137`) — bloqueado em produção pelo fail-closed, perigoso em staging rodando como development.
5. **Segredo no histórico git**: a auditoria interna (`documents/AUDITORIA_PRONTIDAO_2026-06-12.md`, item C1) registrou chave real do OpenRouter e senha do FalkorDB commitadas em `brain-sentry-backend/src/main/resources/application.yml`. O valor foi removido da working tree no commit 536265e (verificado: grep atual = 0 ocorrências), mas **permanece no histórico desde o commit inicial**; revogação da chave NÃO VERIFICÁVEL por esta auditoria.

**Auth**: JWT HMAC + refresh, RBAC ADMIN/USER/READONLY (`internal/middleware/rbac.go`) aplicado nas rotas administrativas; SSO endpoint existe. Fail-closed em produção confirmado no código: `config.Validate()` recusa boot com JWT secret fraco/default ou demo auth ligado, e força `sslmode=require` (`internal/config/config.go:206-239`); compose de produção sem defaults de segredo (`${VAR:?}`).

**Dados**: 10 migrations up/down em `brain-sentry-go/internal/repository/postgres/migrations/` (000001–000010; o CLAUDE.md aponta `cmd/server/migrations/` — caminho desatualizado). Aplicação via `golang-migrate` no Makefile ou serviço `migrate` do compose de produção — **sem auto-migrate no boot**. Entidades centrais: `tenants`, `users`+`user_roles`, `memories` (+tags, relationships, versions), `audit_logs`, `notes`/`hindsight_notes`, `context_summaries`, `sessions`, `webhooks`, `decisions`, `policies`, `events`. Seed: tenant default na migration 000001; usuário demo lazy. **Banco do zero: NÃO TESTADO** nesta máquina (sem Docker); o caminho existe e é usado pelo CI de imagem (`.github/workflows/`).

## 7. Dívidas, bloqueios e spec drift

- **Spec drift maior**: `EXECUTION_PLAN.md` (v2.2, congelado em 2026-01-19) descreve o stack **Java 25 + Spring Boot 4 + Grok** e marca como TODO fases inteiras que **já existem no Go** (JWT/RBAC, pgvector, GraphRAG, interception, audit, CI, Docker prod). `documents/` (BACKEND_SPECIFICATION, MCP_SERVER_API, SETUP_GUIDE, OVERVIEW-V2) também descrevem o backend Java extinto. Quem ler os planos como fonte de verdade é enganado nos dois sentidos.
- **Docs com comandos inexistentes**: `INSTALL_FOR_AGENTS.md` manda rodar `go run ./cmd/server --migrate-only` e `go run ./cmd/cli seed --demo` — **nenhum dos dois existe** (flags reais: `--rebuild/--confirm-destructive`; CLI não tem `seed`).
- **Código morto/zumbi**: `brain-sentry-backend/` (Java, 118 arquivos) fora de CI/deploy; `SelfCorrectingLLM` e `SlidingWindowEnrichment` construídos e descartados (`main.go:375-378`); middleware `ServiceAuth` sem uso; endpoints `/api/v1/integration/*` stub.
- **CLI sem wiring**: 12 comandos registrados, services nunca injetados em produção — só `init` funciona (verificado por execução).
- **Grafo não em tempo real**: nós Memory só entram no FalkorDB via `--rebuild graph`; create/update não gravam no grafo.
- **Frontend**: sem `package-lock.json` (npm ci quebra; builds não-reproduzíveis); `npm test` quebrado (vitest sem config coleta specs do Playwright); zero testes unitários; bundle único 2.37 MB.
- **Sem ADRs/OpenSpec**: previsto no plano (DOC-004 TODO), nunca criado.
- **TODOs no código**: praticamente zero (grep limpo — a dívida está nos docs, não em comentários).
- ADRs de plugin: `plugins/claude-code/plugin.json` aponta porta/rota MCP erradas; parado desde 2026-04-23.

## 8. Prontidão para o VendaX

**O que o VendaX consegue usar HOJE** (dado infra Postgres+FalkorDB+Redis + chave LLM):
- **Memória de longo prazo por REST**: CRUD, busca híbrida, bi-temporal (as-of/changed-since), trust/provenance, conflitos, upload de documentos. É o núcleo mais maduro (1281 testes unitários verdes, homologação interna registrada).
- **MCP server com 7 tools** — um agente de vendas (Claude/LangChain com cliente MCP) consome memória diretamente via `/v1/mcp/message` com JWT. Superfície pronta em código; falta apenas validação runtime.
- **`POST /v1/intercept`** — injeção automática de contexto com PII masking e anti-injection, útil como middleware de prompt do copilot.
- Admin UI completa para operar/curar memórias (34 telas, 74 E2E verdes).

**O que precisa de trabalho antes de o VendaX depender**:
1. **Validação runtime end-to-end** (subir infra, rodar E2E real + explorer validate + benchmark) — nada disso pôde ser exercitado nesta auditoria por falta de Docker.
2. **Grafo em tempo real**: hoje o knowledge graph de memórias só existe após rebuild manual — para um copiloto de vendas com grafo vivo (cliente↔pedido↔visita), o create precisa gravar no FalkorDB.
3. **Chaves LLM por tenant** — hoje globais; multi-cliente VendaX compartilharia quota/custo.
4. **Conectores**: registry vazio; se o VendaX quiser ingestão de fontes externas, é retrabalho.
5. **Durabilidade de mesh/actions/traces** (in-memory) se multi-agente for usado.
6. Higiene: revogar/confirmar revogação da chave no histórico git, commitar lockfile do frontend, corrigir plugin MCP e docs de instalação.

**Estimativa grosseira para "homologável" como serviço de memória do VendaX** (itens 1, 2, 3 e 6): **3–5 semanas-pessoa**. Com conectores e mesh durável (itens 4–5): +3–4 semanas-pessoa.

**Resposta à pergunta específica (Brain Sentry)** — *"o grafo de memória (FalkorDB) grava e recupera de verdade? Há API/tool consumível por um agente hoje?"*
- **Gravação/leitura no grafo**: o código é real, não fachada — `MERGE (m:Memory {id:...})` em `memory_graph.go:31-35`, multi-hop `MATCH path=(seed:Memory)-[r:RELATED_TO*1..N]-(target)` em `graph_rag.go:71-89`, índice vetorial cosine criado lazy (`main.go:104-113`). **Porém** a gravação de memórias no grafo acontece **apenas via rebuild** (`internal/rebuild/targets.go:87`, comando `--rebuild graph`); no caminho quente de criação, o service só faz `VectorSearch` no grafo. Escrita em tempo real existe só para entidades (`/v1/entity-graph/extract`). Verificação em runtime **NÃO FOI POSSÍVEL** nesta máquina (sem Docker → sem FalkorDB); a evidência é código + testes unitários do repositório de grafo.
- **API/tool consumível por agente hoje**: **sim, em código funcional** — MCP com 7 tools/3 resources/6 prompts sob JWT (`internal/mcp/`), Semantic API (`/v1/remember|recall`), `/v1/intercept` e REST completo. O plugin Claude Code existente está com drift (porta/rota erradas) e precisaria de ~1 dia de correção.

## 9. Resumo executivo em 5 linhas

1. Projeto ativo (último commit 2026-06-28, 1 dev), reescrito de Java para Go; o núcleo de memória (CRUD, busca híbrida, bi-temporal, trust, MCP) é código funcional com 1281 testes Go verdes e 74 E2E de frontend verdes — verificado por execução nesta auditoria.
2. Runtime completo NÃO foi validado aqui (sem Docker na máquina): server faz fail-fast sem Postgres; FalkorDB/MCP/webhooks ao vivo ficam como evidência de código, não de execução.
3. Maior surpresa negativa: o grafo de memórias só é populado via rebuild manual (não em tempo real) e o CLI é esqueleto sem services cabeados; conectores externos são semi-stub.
4. Segurança madura para o estágio (tenancy por coluna, RBAC, fail-closed, PII, anti-injection), com riscos pontuais: tenant default fail-open, grafo único FalkorDB, chave LLM global, segredo antigo no histórico git.
5. Para o VendaX: a superfície REST+MCP de memória é aproveitável praticamente como está; 3–5 semanas-pessoa fecham os gaps para homologação como serviço de memória do copilot.

---

```yaml
baseline:
  projeto: "Brain Sentry (brainsentry.io)"
  data: "2026-07-02"
  ultimo_commit: "2026-06-28"
  build_ok: true
  testes: "1355 passando (1281 Go + 74 E2E Playwright mock) / 0 falhando / 1 pulado (+2 arquivos de integração Go não executados por falta de Docker; 0 testes unitários de frontend)"
  sobe_localmente: nao_testado   # sem Docker na máquina de auditoria; server verificado fail-fast sem Postgres; frontend dev sobe (porta 5173)
  maturidade_geral: 4
  capabilities:
    - nome: "Memória CRUD + busca híbrida (pgvector/BM25/recência)"
      maturidade: 4
      estado: "funcional"
    - nome: "Interceptação de prompt (injeção de contexto + PII + anti-injection)"
      maturidade: 4
      estado: "funcional"
    - nome: "Bi-temporal (as-of, changed-since, supersessão, staleness)"
      maturidade: 4
      estado: "funcional"
    - nome: "Trust/provenance + resolução de conflitos"
      maturidade: 4
      estado: "funcional"
    - nome: "Ingestão de documentos (txt/md/csv/json/docx)"
      maturidade: 4
      estado: "funcional"
    - nome: "MCP server (7 tools, JSON-RPC 2.0 + SSE)"
      maturidade: 4
      estado: "funcional"
    - nome: "Knowledge graph FalkorDB (GraphRAG, entidades, comunidades, NL->Cypher)"
      maturidade: 3
      estado: "funcional"   # memórias entram no grafo só via rebuild, não em tempo real
    - nome: "Modelo cognitivo (auto-forget, spreading activation, reflexão)"
      maturidade: 3
      estado: "funcional"
    - nome: "Governança (decisions, policies, events, raciocínio abdutivo)"
      maturidade: 3
      estado: "funcional"
    - nome: "Frontend admin (34 telas, i18n, 74 E2E verdes)"
      maturidade: 4
      estado: "funcional"
    - nome: "Multi-tenancy + RBAC + fail-closed"
      maturidade: 4
      estado: "funcional"
    - nome: "Webhooks"
      maturidade: 3
      estado: "funcional"
    - nome: "TUI (Bubble Tea)"
      maturidade: 3
      estado: "funcional"
    - nome: "Explorer + benchmark de retrieval"
      maturidade: 3
      estado: "funcional"
    - nome: "Mesh P2P / Actions / Traces (estado in-memory)"
      maturidade: 3
      estado: "parcial"
    - nome: "CLI (12 comandos, services não cabeados)"
      maturidade: 2
      estado: "parcial"
    - nome: "Conectores externos (GitHub/Notion/Drive)"
      maturidade: 2
      estado: "parcial"
    - nome: "Plugin Claude Code (drift de porta/rota MCP)"
      maturidade: 2
      estado: "parcial"
    - nome: "Backend Java legado"
      maturidade: 0
      estado: "parcial"   # morto; fora de CI/deploy
  superficies_consumiveis_hoje:
    - "REST /v1/memories (CRUD, search, upload, as-of, changed-since, trust)"
    - "REST /v1/intercept (injeção de contexto)"
    - "REST /v1/remember | /v1/recall | /v1/improve | /v1/forget"
    - "MCP /v1/mcp/message|sse|batch — tools: intercept_prompt, create_memory, search_memories, get_memory, list_memories, update_memory, delete_memory"
    - "REST /v1/graph/* e /v1/entity-graph/* (requer FalkorDB)"
    - "REST /v1/conflicts/*, /v1/decisions, /v1/policies, /v1/events"
    - "REST /v1/webhooks (push outbound)"
    - "Prometheus /metrics, /health, /swagger.json"
  bloqueadores_criticos:
    - "Runtime não validado nesta auditoria (sem Docker): Postgres/FalkorDB/MCP ao vivo não exercitados"
    - "Grafo de memórias só populado via rebuild manual — sem escrita em tempo real no create"
    - "Chaves LLM globais por servidor (não configuráveis por tenant)"
    - "Chave OpenRouter presente no histórico git (removida da working tree; revogação não confirmada)"
    - "Conectores externos com registry vazio (semi-stub); CLI sem services cabeados"
  esforco_para_homologavel: "3-5 semanas-pessoa (núcleo memória+MCP para o VendaX); +3-4 para conectores e mesh durável"
  pronto_para_vendax: "parcial"
```
