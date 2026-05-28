# Swarm deploy — DevOps VPS

Steps to get **brainsentry-backend** + **brainsentry-frontend** running
on the Integrall DevOps Docker Swarm, sharing its existing Redis +
FalkorDB but with a **dedicated** Postgres (with pgvector) so we don't
touch the shared `postgres` service.

The CI workflows in `.github/workflows/build-{backend,frontend}-image.yml`
publish images to `ghcr.io/integrall-tech/brainsentry-{backend,frontend}`
on every push to `main` (and on version tags). The swarm pulls these
images on `docker stack deploy`.

---

## Pre-flight

You'll do these once. Steps assume you have shell access to the swarm
manager and `docker` permission there.

### 1. Generate secrets in the DevOps repo

In the same dir as your existing `secrets/*.txt`:

```bash
cd /path/to/devops-stack

# Postgres password for the dedicated brainsentry DB
openssl rand -base64 32 | tr -d '/+=' | head -c 24 > secrets/brainsentry_postgres_password.txt

# JWT signing key for the backend (must be at least 32 chars)
openssl rand -base64 48 > secrets/brainsentry_jwt_secret.txt

# OpenRouter PAT (or LiteLLM master key if routing via your existing
# litellm service). Used as Bearer for the LLM provider.
echo 'sk-or-v1-...' > secrets/brainsentry_llm_api_key.txt

chmod 600 secrets/brainsentry_*.txt
```

### 2. Make the GHCR packages pullable from the swarm

After the first CI run publishes them, the packages will exist in the
`integrall-tech` org as **private** by default. Either:

- **Make public**: `https://github.com/orgs/integrall-tech/packages` → click
  `brainsentry-backend` → Package settings → Change visibility → Public.
  Same for `brainsentry-frontend`. Easiest, no auth on pull.
- **Keep private** + give the swarm a PAT: `docker login ghcr.io` on the
  manager node using a PAT with `read:packages` for the org. Then add
  `--with-registry-auth` to the `docker stack deploy` invocation so the
  swarm propagates the credential to worker nodes.

---

## Stack additions

Add the snippet below to `secrets:` and `services:` in your `stack.yml`.

### Secrets

```yaml
secrets:
  # ... existing secrets ...
  brainsentry_postgres_password:
    file: ./secrets/brainsentry_postgres_password.txt
  brainsentry_jwt_secret:
    file: ./secrets/brainsentry_jwt_secret.txt
  brainsentry_llm_api_key:
    file: ./secrets/brainsentry_llm_api_key.txt
```

### Services

```yaml
services:
  # ... existing services ...

  # ==========================================================================
  # BRAINSENTRY — Dedicated Postgres (pgvector required by migration 8)
  # ==========================================================================
  brainsentry-postgres:
    image: pgvector/pgvector:pg18
    hostname: brainsentry-postgres
    environment:
      - POSTGRES_USER=brainsentry
      - POSTGRES_PASSWORD_FILE=/run/secrets/brainsentry_postgres_password
      - POSTGRES_DB=brainsentry
      - PGDATA=/var/lib/postgresql/data/pgdata
      - TZ=${TZ:-America/Sao_Paulo}
    secrets:
      - brainsentry_postgres_password
    volumes:
      - ~/apps/data/brainsentry-postgres:/var/lib/postgresql/data
      - ~/apps/logs/brainsentry-postgres:/var/log/postgresql
      - ~/apps/backups/brainsentry-postgres:/backups
    ports:
      - "${BRAINSENTRY_POSTGRES_PORT:-5445}:5432"
    networks:
      - devops
    deploy:
      mode: replicated
      replicas: 1
      resources:
        limits: { cpus: '1.0', memory: 1G }
        reservations: { memory: 512M }
    labels:
      - "com.devops.stack.category=database"
      - "com.devops.stack.type=sql"
      - "com.devops.stack.description=PostgreSQL 18 + pgvector dedicated to brainsentry"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U brainsentry -d brainsentry"]
      interval: 10s
      timeout: 5s
      retries: 5
    logging:
      driver: "json-file"
      options: { max-size: "10m", max-file: "3" }

  # ==========================================================================
  # BRAINSENTRY — Backend (Go API)
  # ==========================================================================
  brainsentry-backend:
    image: ghcr.io/integrall-tech/brainsentry-backend:latest
    hostname: brainsentry-backend
    ports:
      - "${BRAINSENTRY_BACKEND_PORT:-8081}:8081"
    environment:
      - DB_HOST=brainsentry-postgres
      - DB_PORT=5432
      - DB_NAME=brainsentry
      - DB_USER=brainsentry
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - FALKORDB_HOST=falkordb
      - FALKORDB_PORT=6379
      - AI_MODEL=${BRAINSENTRY_AI_MODEL:-anthropic/claude-haiku-4-5}
      - CORS_ORIGINS=${BRAINSENTRY_CORS_ORIGINS:-http://localhost:3000,https://*.integrall.tech}
      - TZ=${TZ:-America/Sao_Paulo}
    # The Go binary reads passwords from env vars, not secret files. The
    # shell wrapper exports each secret into the process before exec'ing
    # the binary so signals (SIGTERM on swarm stop) still reach it.
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        export DB_PASSWORD="$$(cat /run/secrets/brainsentry_postgres_password)"
        export REDIS_PASSWORD="$$(cat /run/secrets/redis_password)"
        export FALKORDB_PASSWORD="$$(cat /run/secrets/falkordb_password)"
        export JWT_SECRET="$$(cat /run/secrets/brainsentry_jwt_secret)"
        export BRAINSENTRY_AI_AGENTIC_MODEL_API_KEY="$$(cat /run/secrets/brainsentry_llm_api_key)"
        exec /app/brainsentry
    secrets:
      - brainsentry_postgres_password
      - redis_password
      - falkordb_password
      - brainsentry_jwt_secret
      - brainsentry_llm_api_key
    networks:
      - devops
    deploy:
      mode: replicated
      replicas: 1
      restart_policy:
        condition: on-failure
        max_attempts: 5
      resources:
        limits: { cpus: '2.0', memory: 1G }
        reservations: { memory: 512M }
    depends_on:
      - brainsentry-postgres
      - redis
      - falkordb
    labels:
      - "com.devops.stack.category=application"
      - "com.devops.stack.description=Brain Sentry Go backend API"
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8081/api/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options: { max-size: "10m", max-file: "3" }

  # ==========================================================================
  # BRAINSENTRY — Frontend (React SPA + nginx + /api proxy)
  # ==========================================================================
  brainsentry-frontend:
    image: ghcr.io/integrall-tech/brainsentry-frontend:latest
    hostname: brainsentry-frontend
    ports:
      - "${BRAINSENTRY_FRONTEND_PORT:-8086}:80"
    environment:
      - TZ=${TZ:-America/Sao_Paulo}
    networks:
      - devops
    deploy:
      mode: replicated
      replicas: 1
      restart_policy:
        condition: on-failure
        max_attempts: 5
      resources:
        limits: { cpus: '0.3', memory: 128M }
        reservations: { memory: 64M }
    depends_on:
      - brainsentry-backend
    labels:
      - "com.devops.stack.category=application"
      - "com.devops.stack.description=Brain Sentry React frontend"
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost/"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging:
      driver: "json-file"
      options: { max-size: "10m", max-file: "3" }
```

---

## First-time deploy

### 1. Deploy the dedicated Postgres first

We need it healthy before we can apply migrations:

```bash
docker stack deploy -c stack.yml --with-registry-auth devops
docker service ps devops_brainsentry-postgres --no-trunc

# Wait for healthcheck to pass
until docker exec $(docker ps -q -f name=devops_brainsentry-postgres) \
    pg_isready -U brainsentry -d brainsentry; do sleep 2; done
```

### 2. Apply migrations 1–9

The image ships migrations at `/app/migrations/`. The binary does NOT
auto-run them — apply manually with `psql`. From a host with network
access to the dedicated Postgres (e.g. the swarm manager, port 5445 by
default):

```bash
# Install pgvector extension first (migration 8 needs it).
PGPASSWORD="$(cat secrets/brainsentry_postgres_password.txt)" psql \
  -h <swarm_manager_ip> -p 5445 -U brainsentry -d brainsentry \
  -c "CREATE EXTENSION IF NOT EXISTS vector;"

# Then apply every up migration in order.
for n in 000001 000002 000003 000004 000005 000006 000007 000008 000009; do
  PGPASSWORD="$(cat secrets/brainsentry_postgres_password.txt)" psql \
    -h <swarm_manager_ip> -p 5445 -U brainsentry -d brainsentry \
    -v ON_ERROR_STOP=1 \
    -f /tmp/migrations/${n}*.up.sql
done
```

The migration files are also in this repo under
`brain-sentry-go/internal/repository/postgres/migrations/`. Copy them to
the host or run them straight from the repo.

### 3. Verify backend health

```bash
curl -fsS http://<swarm_manager_ip>:8081/api/health
# {"status":"UP"}

curl -s http://<swarm_manager_ip>:8081/api/v1/diagnostics | jq '.checks[] | {name, status}'
# postgres ok, redis ok, falkordb ok (all from the swarm)
```

### 4. Verify frontend

```bash
curl -fsS http://<swarm_manager_ip>:8086/
# returns the SPA index.html

# /api/* on the frontend proxies into the backend:
curl -fsS http://<swarm_manager_ip>:8086/api/health
# {"status":"UP"}
```

### 5. Run the validation suite against the live stack (optional)

From the dev machine that has `brain-sentry-explorer/`:

```bash
cd brain-sentry-explorer
echo "BS_BASE_URL=http://<swarm_manager_ip>:8081/api" > .env
npm install
npm run validate
# expect 68/73 passed, 5 skipped (or all 73 if you also expose the legacy
# /v1/decisions stack — see capabilities probe output)
```

---

## Update flow

After the first deploy, the normal cycle is:

1. Merge a PR to `main` → CI builds the affected image(s) and pushes to
   GHCR with `:latest` + `:<sha>` tags.
2. On the swarm manager:
   ```bash
   docker service update --image ghcr.io/integrall-tech/brainsentry-backend:latest \
     --with-registry-auth devops_brainsentry-backend
   # or for frontend:
   docker service update --image ghcr.io/integrall-tech/brainsentry-frontend:latest \
     --with-registry-auth devops_brainsentry-frontend
   ```
3. If the PR added a new migration, run it with the same `psql` loop
   from step 2 above before pointing the new backend at the DB.

For reproducible rollouts pin a specific `:<sha>` tag instead of
`:latest` in `stack.yml`, then bump the SHA on each deploy.

---

## Knobs (env vars on the brainsentry-backend service)

| Variable | Default | Notes |
|---|---|---|
| `BRAINSENTRY_BACKEND_PORT` | 8081 | Host port published by the service |
| `BRAINSENTRY_FRONTEND_PORT` | 8086 | Host port for the SPA |
| `BRAINSENTRY_POSTGRES_PORT` | 5445 | Host port for the dedicated DB |
| `BRAINSENTRY_AI_MODEL` | `anthropic/claude-haiku-4-5` | OpenRouter model id |
| `BRAINSENTRY_CORS_ORIGINS` | `http://localhost:3000,https://*.integrall.tech` | Comma-separated; what the SPA may call from |

To switch the LLM provider entirely (e.g. point at the internal
`litellm` service instead of OpenRouter direct), set in the service
`environment`:

```yaml
- AI_BASE_URL=http://litellm:4000/v1
- AI_MODEL=<modelo configurado no litellm/config.yaml>
```

and ship the LiteLLM master key in the `brainsentry_llm_api_key` secret
instead of the OpenRouter PAT.

---

## Troubleshooting

- **`relation "decisions" does not exist`** — migration 8 wasn't
  applied. Re-run the migration loop. Verify with
  `\dt` on the dedicated DB.
- **Backend logs `redis: connection refused`** — the brainsentry-backend
  service can't see the shared `redis` service. Check both are on the
  `devops` overlay network.
- **Frontend `/api` 502** — backend isn't healthy yet. Watch
  `docker service logs devops_brainsentry-backend`.
- **CI workflow fails at push step** — make sure
  `Settings → Actions → General → Workflow permissions` is set to
  *Read and write permissions* on the repo.
