#!/usr/bin/env bash
set -euo pipefail

DEPLOY_BASE_DIR="${DEPLOY_BASE_DIR:-/opt/video-saas}"
AWS_REGION="${AWS_REGION:-us-east-1}"

dnf update -y
dnf install -y awscli docker ruby wget

systemctl enable docker
systemctl start docker
usermod -aG docker ec2-user

mkdir -p /usr/local/lib/docker/cli-plugins
curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

mkdir -p "${DEPLOY_BASE_DIR}/shared"/{env,postgres,redis,rabbitmq,outputs,artifacts,storage,caddy/data,caddy/config}
mkdir -p "${DEPLOY_BASE_DIR}/release"

cd /tmp
wget "https://aws-codedeploy-${AWS_REGION}.s3.${AWS_REGION}.amazonaws.com/latest/install"
chmod +x ./install
./install auto
systemctl enable codedeploy-agent
systemctl start codedeploy-agent

echo "Bootstrap complete. Re-login so ec2-user picks up the docker group."
