#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="${1:-$(cd "$(dirname "$0")/../../.." && pwd)}"
DEPLOY_BASE_DIR="${DEPLOY_BASE_DIR:-/opt/video-saas}"
GLOBAL_ENV="${DEPLOY_BASE_DIR}/shared/env/global.env"

if [[ ! -f "${GLOBAL_ENV}" ]]; then
  echo "missing global env: ${GLOBAL_ENV}" >&2
  exit 1
fi

required_files=(
  "${GLOBAL_ENV}"
  "${DEPLOY_BASE_DIR}/shared/env/backend.env"
  "${DEPLOY_BASE_DIR}/shared/env/frontend.env"
  "${SOURCE_DIR}/docker-compose.bootstrap.yml"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "${file}" ]]; then
    echo "missing required file: ${file}" >&2
    exit 1
  fi
done

source "${GLOBAL_ENV}"

docker build \
  -f "${SOURCE_DIR}/backend/Dockerfile.prod" \
  -t "video-saas/backend-bootstrap:latest" \
  "${SOURCE_DIR}/backend"

docker build \
  -f "${SOURCE_DIR}/frontend/Dockerfile.prod" \
  -t "video-saas/frontend-bootstrap:latest" \
  "${SOURCE_DIR}/frontend"

docker compose \
  --project-name "${COMPOSE_PROJECT_NAME:-video-saas}" \
  --env-file "${GLOBAL_ENV}" \
  -f "${SOURCE_DIR}/docker-compose.bootstrap.yml" \
  up -d --no-build

docker compose \
  --project-name "${COMPOSE_PROJECT_NAME:-video-saas}" \
  --env-file "${GLOBAL_ENV}" \
  -f "${SOURCE_DIR}/docker-compose.bootstrap.yml" \
  ps
