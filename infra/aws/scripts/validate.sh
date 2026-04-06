#!/usr/bin/env bash
set -euo pipefail

DEPLOY_BASE_DIR="${DEPLOY_BASE_DIR:-/opt/video-saas}"
RELEASE_DIR="${DEPLOY_BASE_DIR}/release"
GLOBAL_ENV="${DEPLOY_BASE_DIR}/shared/env/global.env"
IMAGE_ENV="${RELEASE_DIR}/infra/aws/image-tags.env"

source "${GLOBAL_ENV}"

compose_args=(
  --project-name "${COMPOSE_PROJECT_NAME:-video-saas}"
  --env-file "${GLOBAL_ENV}"
  --env-file "${IMAGE_ENV}"
  -f "${RELEASE_DIR}/docker-compose.prod.yml"
)

if [[ "${ENABLE_WORKER_STACK:-false}" == "true" ]]; then
  compose_args+=(-f "${RELEASE_DIR}/docker-compose.worker.prod.yml")
fi

compose() {
  docker compose "${compose_args[@]}" "$@"
}

backend_container="$(compose ps -q backend)"
frontend_container="$(compose ps -q frontend)"

if [[ -z "${backend_container}" || -z "${frontend_container}" ]]; then
  echo "frontend or backend container is missing" >&2
  exit 1
fi

backend_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${backend_container}")"
frontend_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${frontend_container}")"

if [[ "${backend_health}" != "healthy" ]]; then
  echo "backend is not healthy: ${backend_health}" >&2
  exit 1
fi

if [[ "${frontend_health}" != "healthy" ]]; then
  echo "frontend is not healthy: ${frontend_health}" >&2
  exit 1
fi

echo "deployment validated"
