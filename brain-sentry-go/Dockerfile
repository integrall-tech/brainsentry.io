# syntax=docker/dockerfile:1
#
# Multi-stage build for the brain-sentry-go server binary.
#
# Build context MUST be ./brain-sentry-go (matches what
# .github/workflows/build-backend-image.yml uses as `context:`).
#
# Runtime configuration: the baked config.yaml carries dev defaults
# (DB on localhost, etc). In production every host/credential is
# overridden via env vars consumed by internal/config/config.go's
# applyEnvOverrides — see docs/deploy/swarm-deploy.md for the full
# matrix of DB_HOST / REDIS_* / FALKORDB_* / JWT_SECRET /
# BRAINSENTRY_AI_AGENTIC_MODEL_API_KEY / AI_MODEL / CORS_ORIGINS.

FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads in a dedicated layer.
COPY go.mod go.sum ./
RUN go mod download

# Bring in source and build a stripped, statically-linked binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /out/brainsentry ./cmd/server

# --- Runtime ---
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata curl \
    && addgroup -S brainsentry && adduser -S -G brainsentry brainsentry

WORKDIR /app
COPY --from=build --chown=brainsentry:brainsentry /out/brainsentry /app/brainsentry
# config.yaml is the baseline; env vars override host/credentials.
COPY --chown=brainsentry:brainsentry config.yaml /app/config.yaml
# Migrations ship with the image so operators can apply them via psql
# (the binary itself does not auto-migrate — see deploy guide).
COPY --chown=brainsentry:brainsentry internal/repository/postgres/migrations /app/migrations

USER brainsentry
EXPOSE 8081

# brainsentry exposes /api/health (server.context_path = /api by default).
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD curl -fsS http://localhost:8081/api/health || exit 1

ENTRYPOINT ["/app/brainsentry"]
