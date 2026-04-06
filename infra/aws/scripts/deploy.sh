#!/usr/bin/env bash
set -euo pipefail

DEPLOY_BASE_DIR="${DEPLOY_BASE_DIR:-/opt/video-saas}"
RELEASE_DIR="${DEPLOY_BASE_DIR}/release"
GLOBAL_ENV="${DEPLOY_BASE_DIR}/shared/env/global.env"
IMAGE_ENV="${RELEASE_DIR}/infra/aws/image-tags.env"

source "${GLOBAL_ENV}"
source "${IMAGE_ENV}"

if [[ -z "${AWS_REGION:-}" ]]; then
  echo "AWS_REGION must be set in ${GLOBAL_ENV}" >&2
  exit 1
fi

if [[ -z "${ECR_REGISTRY:-}" ]]; then
  echo "ECR_REGISTRY must be set in ${GLOBAL_ENV}" >&2
  exit 1
fi

aws ecr get-login-password --region "${AWS_REGION}" | docker login --username AWS --password-stdin "${ECR_REGISTRY}"

compose_args=(
  --project-name "${COMPOSE_PROJECT_NAME:-video-saas}"
  --env-file "${GLOBAL_ENV}"
  --env-file "${IMAGE_ENV}"
  -f "${RELEASE_DIR}/docker-compose.prod.yml"
)

if [[ "${ENABLE_WORKER_STACK:-false}" == "true" ]]; then
  compose_args+=(-f "${RELEASE_DIR}/docker-compose.worker.prod.yml")
fi

docker compose "${compose_args[@]}" pull
docker compose "${compose_args[@]}" up -d --remove-orphans
