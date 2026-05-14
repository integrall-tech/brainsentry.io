# INSTALL_FOR_AGENTS.md — instalação assistida por agente

> Você é um agente de IA preparando um ambiente novo para rodar o
> **brainsentry.io**. Siga estes 9 passos. **Não pule** o passo 3.5.

---

## Pré-requisitos

- macOS 13+, Linux, ou WSL2
- Docker + Docker Compose
- Go 1.25+
- Node 20+ (idealmente via nvm)
- `git`, `curl`, `psql` (cliente)
- Acesso a uma chave OpenRouter ou Anthropic (para testes integrados; mocks
  cobrem boa parte da suite)

```bash
docker --version && go version && node --version
```

Se algo falhar, **pare e peça ao operador** para instalar o que falta.

---

## Passo 1 — Clonar e inspecionar

```bash
git clone <repo> brainsentry.io
cd brainsentry.io
ls -la
```

Esperado: `AGENTS.md`, `CLAUDE.md`, `README.md`, `brain-sentry-go/`,
`brain-sentry-frontend/`, `documents/`.

Leia `AGENTS.md` por completo antes de continuar. Se você acabou de chegar
neste projeto, leia também `CLAUDE.md`.

---

## Passo 2 — Subir a infra de dev

```bash
cd brain-sentry-go
docker compose up -d
docker compose ps
```

Esperado: serviços `postgres` (5432), `falkordb` (6379-graph), `redis` (6380)
todos `Up (healthy)`. Se algum não subiu, `docker compose logs <svc>`.

---

## Passo 3 — Migrations + seed

```bash
cd brain-sentry-go
go run ./cmd/server --migrate-only   # cria schema; sai depois
go run ./cmd/cli seed --demo         # cria tenant demo + user demo@example.com
```

Esperado: `migration applied: 0023_...`, `demo user ensured`.

---

## Passo 3.5 — **PARE E PERGUNTE AO OPERADOR**

Antes de continuar, confirme com o operador:

1. **Provider de LLM**: OpenRouter (default) ou Anthropic direto? A escolha
   muda `config.yaml` `llm_provider:` e o env var necessário.
2. **Modo de embeddings**: OpenAI (default) ou local? Se local, vai precisar
   de Ollama.
3. **Quer dados de teste reais ou só mocks?** A suite E2E tem dois modos:
   `npm run test:e2e` (com mocks, default) e `npm run test:e2e:real`
   (precisa do backend rodando e API keys reais).

Não invente as respostas. Pergunte.

---

## Passo 4 — Variáveis de ambiente

```bash
cd brain-sentry-go
cp config.yaml.example config.yaml   # se existir
# edite config.yaml com a chave do passo 3.5
```

Esperado em `config.yaml`:

```yaml
llm:
  provider: openrouter            # ou anthropic
  api_key: ${OPENROUTER_API_KEY}  # via env
embedding:
  provider: openai
  api_key: ${OPENAI_API_KEY}
cors:
  allowed_origins:
    - http://localhost:5173
    - http://localhost:5174
    - http://localhost:5175
```

```bash
export OPENROUTER_API_KEY=sk-or-v1-...
export OPENAI_API_KEY=sk-...
```

---

## Passo 5 — Subir o backend

```bash
cd brain-sentry-go
go run ./cmd/server
```

Esperado: `server listening on :8080`. Em outra aba:

```bash
curl -s http://localhost:8080/v1/health | jq .
# {"status":"healthy","db":"ok","graph":"ok","redis":"ok"}
```

Se algum subsystem está `down`, **não continue** — debug com `docker compose
logs <svc>` e/ou `go run ./cmd/server -v`.

---

## Passo 6 — Subir o admin

```bash
cd brain-sentry-frontend
npm install
npm run dev -- --port 5175 --strictPort
```

Esperado: `Local: http://localhost:5175`. Abra no browser, deve ver tela
de login.

---

## Passo 7 — Login e smoke test

1. Click "Demo Login" (ou `demo@example.com` / `demo`).
2. Você cai no dashboard. Sidebar tem 7 grupos colapsáveis.
3. Click "Memórias" → veja a lista vazia.
4. Click "Console" → digite "test" no recall → veja `no relevant memories`.

Se o login falha com erro 401, é provavelmente CORS — confira passo 4
allowed_origins. Se falha com 500, é o backend (cheque logs).

---

## Passo 8 — Rodar testes

```bash
# Backend
cd brain-sentry-go
go test ./... -count=1 -short

# Frontend (build)
cd ../brain-sentry-frontend
npm run build

# E2E
npx playwright install --with-deps   # primeira vez
npx playwright test --project=chromium
```

Esperado: todos verdes. Se algum E2E flakea, rode novamente — testes que
dependem de toast (mesh) podem ser sensíveis. Se segue vermelho, o teste
real está quebrado.

---

## Passo 9 — Antes de declarar pronto

Cheque:

- [ ] `curl localhost:8080/v1/health` retorna `healthy` em todos os
      subsystems.
- [ ] Login na admin funciona, dashboard renderiza.
- [ ] `go test ./... -short -count=1` verde.
- [ ] `npx playwright test --project=chromium` verde.
- [ ] Você documentou para o operador qual API key foi usada (não
      committe a key!).

---

## Quando algo quebra

1. **Backend não sobe**: confira `docker compose ps`. Se `postgres` está
   restartando, provavelmente é volume corrompido — `docker compose down -v`
   (destrutivo!) e refaça do passo 2.
2. **Admin pede para rebuild**: `rm -rf node_modules .vite && npm install`.
3. **E2E timeout em login**: backend está down. Suba no passo 5.
4. **Erro `pgvector not found`**: imagem do postgres não tem extensão.
   Use `pgvector/pgvector:pg16` no `docker-compose.yml`.

Se nada disso resolve: **pare e peça ajuda** ao operador, com o output
completo do erro.
