#!/usr/bin/env bash
# Gate de release: roda a suíte E2E real-API (real-*.spec.ts) contra um
# ambiente vivo (homologação ou produção pós-deploy).
#
# A suíte sobe o frontend localmente (vite dev) apontando para o backend
# informado e exercita CRUD de memória com versionamento, filtragem de
# interceptação e auto-forget dry-run — dados reais, sem mocks.
#
# Uso:
#   scripts/release-gate.sh https://homolog.suaempresa.com/api
#   scripts/release-gate.sh http://192.168.0.10:8081/api
#
# Pré-requisitos: backend saudável no URL informado; npm install feito em
# brain-sentry-frontend. Cria e remove dados de teste no tenant default.
set -euo pipefail

API_BASE="${1:-${E2E_API_BASE:-}}"
if [ -z "$API_BASE" ]; then
  echo "uso: scripts/release-gate.sh <api-base-url>" >&2
  echo "ex.: scripts/release-gate.sh https://homolog.suaempresa.com/api" >&2
  exit 1
fi

echo "==> Health check: ${API_BASE}/health"
if ! curl -fsS --max-time 10 "${API_BASE}/health" >/dev/null; then
  echo "ERRO: backend não respondeu em ${API_BASE}/health" >&2
  exit 1
fi

echo "==> Diagnostics"
curl -fsS --max-time 15 "${API_BASE}/v1/diagnostics" | head -c 400 || true
echo

cd "$(dirname "$0")/../brain-sentry-frontend"
echo "==> E2E real-API contra ${API_BASE}"
E2E_REAL_API=1 E2E_API_BASE="$API_BASE" npx playwright test -c playwright.real.config.ts

echo "==> Gate PASSOU — release liberado."
