# brain-sentry-explorer

Example client and validation harness for the **brainsentry.io** memory API.

It has two faces, both driven from the same typed HTTP client:

- **Interactive explorer** — a TUI that lets you fire every core memory
  endpoint and watch the live request/response. IDs returned by one call
  are chained into later calls automatically, so dependent endpoints
  (`GET /v1/memories/{id}`, relationships, the pluggable store, ...) work
  without copy-pasting UUIDs. This is the "how do I call the API" half.
- **Validation suite** — chained scenarios that assert each response is
  internally consistent and shaped as expected (`total == results.length`,
  version advances after an update, a deleted memory 404s, filter
  endpoints only return matching rows, ...). This is the "is the API
  behaving correctly" half. It runs headless and exits non-zero on any
  failure, so it can gate CI.

This first cut focuses on the **core: memories** — CRUD, search, filters,
versions, feedback, relationships, and the pluggable `/v1/store/memories`
surface. Other API areas (graph, decisions, policies, events, eval, ...)
can be added the same way: an entry in `src/catalog.ts` and a scenario in
`src/scenarios/`.

## Requirements

- Node.js 20+
- A running brainsentry.io backend (`brain-sentry-go`). By default the
  explorer expects it at `http://localhost:8081/api` — the `server.port`
  and `server.context_path` from `brain-sentry-go/config.yaml`.

## Setup

```bash
cd brain-sentry-explorer
npm install
cp .env.example .env   # then edit if your backend is elsewhere
```

`.env` is loaded automatically. Keys:

| Variable        | Default                                  | Meaning                                   |
| --------------- | ---------------------------------------- | ----------------------------------------- |
| `BS_BASE_URL`   | `http://localhost:8081/api`              | Backend base URL incl. context path       |
| `BS_TENANT_ID`  | `a9f814d2-4dae-41f3-851b-8aa3d4706561`   | Tenant the requests run under             |
| `BS_AUTH_MODE`  | `demo`                                   | `demo` (`POST /v1/auth/demo`) or `login`  |
| `BS_EMAIL`      | —                                        | Used when `BS_AUTH_MODE=login`            |
| `BS_PASSWORD`   | —                                        | Used when `BS_AUTH_MODE=login`            |

The default `demo` mode needs no credentials — it calls
`POST /v1/auth/demo`, which creates/returns the seeded demo user.

## Usage

### Interactive explorer

```bash
npm start
```

- `↑` / `↓` — select an endpoint
- `Enter` — fire it; the response (status, latency, JSON body) shows on
  the right
- `[` / `]` — scroll a long response body
- `r` — reset the captured-ID context
- `v` — run the validation suite
- `e` — back to the explorer
- `q` — quit

Endpoints that need an ID (`GET /v1/memories/{id}`, relationships, ...)
are greyed out until you fire a Create endpoint — its returned id is
captured into the context (shown in the bottom bar) and reused.

### Validation suite

In the TUI press `v`, or run it headless:

```bash
npm run validate
```

Headless mode prints a per-step report and exits `0` when every check
passes, non-zero otherwise. The suite seeds its own data (content
prefixed `[bs-explorer]`) and deletes it in a cleanup step, so it is
safe to re-run and leaves the backend clean.

## Layout

```
src/
  config.ts                 env-driven configuration
  enums.ts                  domain enums mirrored from the backend
  catalog.ts                endpoint catalog for the explorer
  cli.tsx                   entry point (TUI vs --validate)
  validate.ts               headless validation runner
  api/
    client.ts               BrainSentryClient — typed HTTP client
    types.ts                DTOs + loose zod schemas
    memories.ts             typed wrappers for the memory endpoints
  scenarios/
    assert.ts               assertion helpers (incl. zod shape checks)
    runner.ts               scenario engine + live events
    memory-scenarios.ts     the 7 memory validation scenarios
  components/                Ink TUI (App, Explorer, Validation, ...)
```

## Validation scenarios

| Scenario        | What it proves                                              |
| --------------- | ----------------------------------------------------------- |
| Lifecycle       | create → get → update → versions → delete → 404 all agree   |
| Deduplication   | near-duplicate POST returns the existing id; distant POST creates a new one (SimHash dedup is documented contract) |
| Search          | search envelope is consistent and recalls seeded memories   |
| Pagination      | page/size echo, `totalPages` math, no rows shared by pages  |
| Filters         | by-category / by-importance return the row and only matches |
| Feedback        | a helpful vote increments the counter and the weight        |
| Relationships   | a link is created, listed, and removed                      |
| Store           | `/v1/store/memories` CRUD + search + 404-after-delete       |
