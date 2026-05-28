# AGENTS.md — guia rápido para agentes de IA

> Você é um agente de IA (Claude Code, Cursor, Aider, OpenAI Codex…) que precisa
> entender o **brainsentry.io** em 5 minutos. Leia este arquivo do começo ao
> fim antes de tocar em qualquer coisa.

## O que é

**brainsentry.io** é uma plataforma de **memória cognitiva** para fleets de
agentes de IA. Resolve "modelos esquecem contexto entre sessões" com:

- Memória bi-temporal (recorded_at, valid_from/valid_to, superseded_by)
- Spreading activation (BFS com decay) sobre knowledge graph
- Auto-forget (TTL + contradição via Jaccard + low-value cleanup)
- Compressão LLM com extração de facts/concepts/narrative
- Multi-tenancy real
- MCP server (JSON-RPC 2.0 + SSE) para integração nativa com agentes

## Stack — leia em ordem

| Camada | Pasta | Linguagem | O que vive aqui |
|---|---|---|---|
| Backend principal | `brain-sentry-go/` | Go 1.25 | API REST, MCP server, services, repositories |
| Backend legado | `brain-sentry-backend/` | Java/Spring | **Não tocar** — só consultar histórico |
| Frontend admin | `brain-sentry-frontend/` | React 19 + Vite | 31 telas, shadcn/ui, i18next pt-BR/en |
| TUI | `brain-sentry-go/cmd/tui/` | Go (Bubble Tea) | Cliente terminal |
| CLI | `brain-sentry-go/cmd/cli/` | Go (cobra) | Comandos administrativos |
| Docs internos | `documents/` | markdown | Visão de produto, fases, especificações |

**Tudo que você implementar fica em `brain-sentry-go/` ou `brain-sentry-frontend/`.**

## Trust boundary (importante!)

Conteúdo que entra via:

1. **CLI local** (`brain-sentry-go/cmd/cli/`) — operador, confiável.
2. **Admin UI** logado — operador, confiável.
3. **MCP/HTTP de fora** — **não confiável**. Toda string injetada em prompts
   precisa passar por `internal/security.Sanitize()` e ser envelopada com
   `security.FrameMemory()`.

Se você for adicionar uma rota nova que recebe conteúdo de terceiros e o
injeta no LLM, o sanitizer é obrigatório. Veja
`internal/service/interception.go` como exemplo já correto.

## Comandos comuns

```bash
# Levantar a infra de dev (postgres + falkordb + redis)
cd brain-sentry-go && docker compose up -d

# Subir o backend Go (porta 8080)
cd brain-sentry-go && go run ./cmd/server

# Subir o admin (porta 5175)
cd brain-sentry-frontend && npm run dev -- --port 5175

# Login: demo@example.com / demo (botão "Demo Login" na tela de login)

# Rodar testes Go (curtos — sem testcontainers)
cd brain-sentry-go && go test ./... -short -count=1

# Rodar a suite E2E (Playwright)
cd brain-sentry-frontend && npx playwright test --project=chromium

# Rodar visual (você abre o browser e vê)
cd brain-sentry-frontend && npx playwright test --headed --project=chromium
```

## Antes de declarar uma tarefa pronta

1. `go build ./...` no `brain-sentry-go/` — sem erros.
2. `go test ./internal/<pacote>/... -count=1 -short` — verde.
3. Se mexeu em frontend: `npm run build` + `npx playwright test` — verde.
4. **Não pule hooks** (`--no-verify` é proibido por convenção).
5. **Não force-push em `main`**. Branch + PR para mudanças significativas.

## Padrões locais

- **Testes obrigatórios**: toda nova service Go precisa de `_test.go`. Toda
  nova tela frontend precisa de spec E2E em `e2e/tests/`.
- **i18n**: todo texto em UI passa por `t("...")` — nada hardcoded. Adicione
  pt-BR **e** en em `src/i18n/locales/`.
- **Help drawer**: telas novas precisam de entrada em
  `src/lib/help/helpContent.ts` (objetivo, problema, como funciona, fluxo,
  regras-chave) — **focar no negócio, sem jargão técnico**.
- **No comments unless necessary**: código deve falar por si. Comentário só
  para explicar *por quê* não-óbvio (workaround, invariante hidden).
- **Sem ✨ emojis** em código/docs/UI. Só se o usuário pedir explicitamente.

## Onde buscar mais profundidade

- `CLAUDE.md` — arquitetura anotada, key files, bugs históricos.
- `INSTALL_FOR_AGENTS.md` — passo a passo de install em ambiente novo.
- `llms.txt` / `llms-full.txt` — TOC parseável + docs inline.
- `documents/00-PROJECT-OVERVIEW.md` — visão de produto.
- `brain-sentry-go/docs/swagger.go` — spec OpenAPI.

## Quando parar e perguntar

Se algo não está em `CLAUDE.md` nem em `documents/` e a decisão é arquitetural
(novo banco, novo provider de LLM, mudança de schema), **pergunte ao
operador antes de mexer**. O sistema é multi-tenant em produção; uma
migration mal pensada quebra todos os clientes ao mesmo tempo.
