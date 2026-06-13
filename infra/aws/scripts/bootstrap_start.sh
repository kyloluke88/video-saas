#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="${1:-$(cd "$(dirname "$0")/../../.." && pwd)}"
DEPLOY_BASE_DIR="${DEPLOY_BASE_DIR:-/opt/video-saas}"
GLOBAL_ENV="${DEPLOY_BASE_DIR}/shared/env/global.env"
FRONTEND_ENV="${DEPLOY_BASE_DIR}/shared/env/frontend.env"
DOCKER_PROGRESS="${BOOTSTRAP_DOCKER_PROGRESS:-plain}"
GO_BUILD_FLAGS="${BOOTSTRAP_GO_BUILD_FLAGS:--p=1}"

log() {
  printf '==> %s\n' "$*"
}

warn() {
  printf 'WARNING: %s\n' "$*" >&2
}

read_mem_mb() {
  awk '/MemTotal:/ { print int($2 / 1024) }' /proc/meminfo
}

read_swap_mb() {
  awk '/SwapTotal:/ { print int($2 / 1024) }' /proc/meminfo
}

read_disk_mb() {
  df -Pm "${DEPLOY_BASE_DIR}" | awk 'NR==2 { print $4 }'
}

timed_build() {
  local label="$1"
  shift
  local started_at
  started_at="$(date +%s)"

  log "${label}"
  "$@"

  local finished_at
  finished_at="$(date +%s)"
  log "${label} finished in $((finished_at - started_at))s"
}

if [[ ! -f "${GLOBAL_ENV}" ]]; then
  echo "missing global env: ${GLOBAL_ENV}" >&2
  exit 1
fi

required_files=(
  "${GLOBAL_ENV}"
  "${DEPLOY_BASE_DIR}/shared/env/backend.env"
  "${FRONTEND_ENV}"
  "${SOURCE_DIR}/docker-compose.bootstrap.yml"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "${file}" ]]; then
    echo "missing required file: ${file}" >&2
    exit 1
  fi
done

source "${GLOBAL_ENV}"
source "${FRONTEND_ENV}"

memory_mb="$(read_mem_mb)"
swap_mb="$(read_swap_mb)"
disk_mb="$(read_disk_mb)"

log "Host resources: mem=${memory_mb}MiB swap=${swap_mb}MiB disk_avail=${disk_mb}MiB"

if (( memory_mb < 1800 )) && (( swap_mb == 0 )); then
  warn "Low-memory host with no swap detected. Source builds may stall or make SSH unresponsive."
fi

if (( disk_mb < 4096 )); then
  warn "Less than 4GiB of free disk is available under ${DEPLOY_BASE_DIR}. Docker builds may fail."
fi

timed_build "Building backend image" \
  docker build \
    --progress="${DOCKER_PROGRESS}" \
    --build-arg "GO_BUILD_FLAGS=${GO_BUILD_FLAGS}" \
    -f "${SOURCE_DIR}/backend/Dockerfile.prod" \
    -t "video-saas/backend-bootstrap:latest" \
    "${SOURCE_DIR}/backend"

timed_build "Building frontend image" \
  docker build \
    --progress="${DOCKER_PROGRESS}" \
    --build-arg "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL:-}" \
    --build-arg "NEXT_PUBLIC_SITE_URL=${NEXT_PUBLIC_SITE_URL:-}" \
    -f "${SOURCE_DIR}/frontend/Dockerfile.prod" \
    -t "video-saas/frontend-bootstrap:latest" \
    "${SOURCE_DIR}/frontend"

log "Starting bootstrap compose stack"
docker compose \
  --project-name "${COMPOSE_PROJECT_NAME:-video-saas}" \
  --env-file "${GLOBAL_ENV}" \
  -f "${SOURCE_DIR}/docker-compose.bootstrap.yml" \
  up -d --no-build

log "Current bootstrap compose status"
docker compose \
  --project-name "${COMPOSE_PROJECT_NAME:-video-saas}" \
  --env-file "${GLOBAL_ENV}" \
  -f "${SOURCE_DIR}/docker-compose.bootstrap.yml" \
  ps
