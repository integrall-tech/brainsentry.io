# syntax=docker/dockerfile:1.6
#
# Multi-stage build for brain-sentry-frontend.
#
# Uses pnpm (via corepack) for installs. npm 10.x consistently hangs
# ~8min on `npm ci` for this dep tree on GHA runners and then trips
# the "Exit handler never called!" bug — pnpm has a different resolver
# that completes the same install in ~10s. The pnpm-lock.yaml is the
# source of truth; package-lock.json is kept only for backwards
# compatibility with developers who still type `npm` locally (CI is
# pnpm-only).
#
# Build context MUST be ./brain-sentry-frontend.
#
# VITE_API_URL is baked at build time by Vite. Default "/api" keeps
# the image env-agnostic — nginx (docker/nginx.conf) proxies /api/*
# to brainsentry-backend on the swarm overlay network. Override via
# --build-arg VITE_API_URL=https://api.example.com/api when the SPA
# needs an external API URL.

FROM node:22-bookworm-slim AS builder
WORKDIR /app

# corepack ships with Node 22 — enable it and pin pnpm so the version
# is reproducible across local + CI + image rebuilds.
ENV PNPM_VERSION=11.4.0
RUN corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate

# Defensive Node memory ceiling — Vite + heavy deps fit comfortably
# under 4GB but the default 1.5GB can fail mid-build.
ENV NODE_OPTIONS=--max-old-space-size=4096

# Install deps first so source-only changes don't bust the install layer.
# BuildKit cache mount on pnpm's store dir reuses fetched tarballs
# across builds (the store is content-addressable so it stays sane).
COPY package.json pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

COPY . .

ARG VITE_API_URL=/api
ARG VITE_APP_VERSION=dev
ARG VITE_APP_COMMIT=unknown
ENV VITE_API_URL=${VITE_API_URL} \
    VITE_APP_VERSION=${VITE_APP_VERSION} \
    VITE_APP_COMMIT=${VITE_APP_COMMIT}
RUN pnpm run build

# --- Runtime ---
FROM nginx:1.27-alpine
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
RUN chown -R nginx:nginx /usr/share/nginx/html

EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
