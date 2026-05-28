# syntax=docker/dockerfile:1
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

FROM node:22-alpine AS builder
WORKDIR /app

# Defensive npm settings — `npm ci` with large dep trees on GHA runners
# regularly trips npm 10.x's "Exit handler never called!" bug under tight
# memory. NODE_OPTIONS bumps V8's old-space ceiling; the NPM_CONFIG_*
# vars silence audit/fund/update-notifier calls that allocate extra
# memory and slow installs.
ENV NODE_OPTIONS=--max-old-space-size=4096 \
    NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_FUND=false \
    NPM_CONFIG_UPDATE_NOTIFIER=false

# Install deps first so source-only changes don't bust the npm-ci layer.
COPY package.json package-lock.json* ./
RUN npm ci --no-audit --no-fund

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
