# syntax=docker/dockerfile:1.6
#
# Multi-stage build for brain-sentry-frontend.
#
# Build context MUST be ./brain-sentry-frontend (matches what
# .github/workflows/build-frontend-image.yml uses as `context:`).
#
# VITE_API_URL is baked at build time by Vite. The default "/api" keeps
# the image environment-agnostic — nginx (see docker/nginx.conf) forwards
# every /api/* request to the brainsentry-backend container on the docker
# swarm overlay network. To point the SPA at an external API URL instead,
# pass `--build-arg VITE_API_URL=https://api.example.com/api`.

FROM node:22-bookworm-slim AS builder
WORKDIR /app

# Defensive npm settings — `npm ci` with this dep tree (Cytoscape,
# ECharts, react-force-graph, Playwright) consistently hangs ~470s on
# GHA runners then trips npm 10.x's "Exit handler never called!" bug.
# The 470s ≈ npm fetch default 300s + 2 retries — points at one slow
# package that times out and the retry logic eventually hits the bug.
# Failure history on this repo: every Frontend E2E run since April has
# hit this. Stack of defenses:
#   1) Debian (glibc) base — node:22-alpine's musl has caused stalls
#      in native postinstalls of this tree.
#   2) Pin npm to >=10.9 — bundled npm on some Node 22.x patches still
#      ships 10.5/10.8.0 where the exit-handler bug lives.
#   3) NODE_OPTIONS bumps V8 old-space.
#   4) NPM_CONFIG_AUDIT/FUND/UPDATE_NOTIFIER off — kills extra HTTP.
#   5) NPM_CONFIG_FETCH_* — much longer timeouts so a slow registry
#      response doesn't time out before completing.
#   6) NPM_CONFIG_MAXSOCKETS=4 — fewer concurrent connections; the GHA
#      runner network seems to choke when npm opens many sockets.
#   7) BuildKit cache mount on /root/.npm so repeat builds reuse cache
#      and only pay the slow fetch once.
ENV NODE_OPTIONS=--max-old-space-size=4096 \
    NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_FUND=false \
    NPM_CONFIG_UPDATE_NOTIFIER=false \
    NPM_CONFIG_FETCH_TIMEOUT=600000 \
    NPM_CONFIG_FETCH_RETRY_MINTIMEOUT=20000 \
    NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=120000 \
    NPM_CONFIG_MAXSOCKETS=4

RUN npm install -g npm@10.9.3

# Install deps first so source-only changes don't bust the npm-ci layer.
COPY package.json package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

# Then source + build (tsc emit + vite build).
COPY . .

ARG VITE_API_URL=/api
ARG VITE_APP_VERSION=dev
ARG VITE_APP_COMMIT=unknown
ENV VITE_API_URL=${VITE_API_URL} \
    VITE_APP_VERSION=${VITE_APP_VERSION} \
    VITE_APP_COMMIT=${VITE_APP_COMMIT}
RUN npm run build

# --- Runtime ---
FROM nginx:1.27-alpine
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
RUN chown -R nginx:nginx /usr/share/nginx/html

EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
