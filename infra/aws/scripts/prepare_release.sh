#!/usr/bin/env bash
set -euo pipefail

DEPLOY_BASE_DIR="${DEPLOY_BASE_DIR:-/opt/video-saas}"

mkdir -p "${DEPLOY_BASE_DIR}/shared"/{env,postgres,redis,rabbitmq,outputs,artifacts,storage,caddy/data,caddy/config}
mkdir -p "${DEPLOY_BASE_DIR}/release"

required_files=(
  "${DEPLOY_BASE_DIR}/shared/env/global.env"
  "${DEPLOY_BASE_DIR}/shared/env/backend.env"
  "${DEPLOY_BASE_DIR}/shared/env/frontend.env"
)

source "${DEPLOY_BASE_DIR}/shared/env/global.env"

if [[ "${ENABLE_WORKER_STACK:-false}" == "true" ]]; then
  required_files+=("${DEPLOY_BASE_DIR}/shared/env/worker.env")
fi

for file in "${required_files[@]}"; do
  if [[ ! -f "${file}" ]]; then
    echo "missing required env file: ${file}" >&2
    exit 1
  fi
done
