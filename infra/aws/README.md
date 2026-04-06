# AWS Single-Host Deployment

This repository now includes a low-cost AWS deployment path built around one x86_64 EC2 instance, Docker Compose, Caddy, and AWS CodePipeline/CodeBuild/CodeDeploy.

## Why this path

- It avoids the fixed monthly cost of ALB + ECS for the first production version.
- It keeps the deployment model close to local Docker Compose.
- It still supports push-to-deploy CI/CD on AWS.
- It stays compatible with the current worker stack. The worker currently prefers x86_64 because MFA is pinned to that path in practice, so do not start with Graviton for this repo.
- The default production stack now excludes `rabbitmq` and `worker`. You can enable them later without redesigning the host.

## Target topology

- One public EC2 instance running:
  - `caddy`
  - `frontend`
  - `backend`
  - `postgres`
  - `redis`
- Optional later:
  - `rabbitmq`
  - `worker`
- One Elastic IP mapped to your domain.
- CodePipeline:
  - Source: GitHub via CodeConnections
  - Build: CodeBuild using [buildspec.yml](/Users/luca/go/github.com/luca/video-saas/buildspec.yml)
  - Deploy: CodeDeploy using [appspec.yml](/Users/luca/go/github.com/luca/video-saas/appspec.yml)

## One-time AWS setup

1. Create an x86_64 Amazon Linux 2023 EC2 instance.
2. Attach an Elastic IP.
3. Open inbound ports `22`, `80`, and `443`.
4. Attach an IAM role that can:
   - pull from ECR
   - read CodeDeploy artifacts
   - read SSM if you later move secrets there
5. Create three ECR repositories:
   - `video-saas-backend`
   - `video-saas-frontend`
   - `video-saas-worker`
6. Point your domain A record to the Elastic IP.
7. SSH into the EC2 instance and run [bootstrap_ec2.sh](/Users/luca/go/github.com/luca/video-saas/infra/aws/scripts/bootstrap_ec2.sh).

## Host directory layout

The EC2 host keeps mutable state under `/opt/video-saas/shared`:

- `env/`
- `postgres/`
- `redis/`
- `caddy/`

Create these additional directories only when `ENABLE_WORKER_STACK=true`:

- `rabbitmq/`
- `outputs/`
- `artifacts/`
- `storage/`

The current release bundle is copied to `/opt/video-saas/release`.

## Required env files on EC2

Create these files under `/opt/video-saas/shared/env/` before the first deployment:

- `global.env`
- `backend.env`
- `frontend.env`

Examples are included:

- [global.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/global.env.example)
- [backend.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/backend.env.example)
- [frontend.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/frontend.env.example)
- [worker.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/worker.env.example)

Only create `worker.env` when you later set `ENABLE_WORKER_STACK=true`.

## Deployment flow

1. Push to GitHub.
2. CodePipeline starts.
3. CodeBuild:
   - runs backend tests
   - runs frontend build
   - builds production Docker images
   - pushes them to ECR
   - emits `infra/aws/image-tags.env`
   - only builds the worker image when `BUILD_WORKER_IMAGE=true`
4. CodeDeploy copies the release bundle to EC2.
5. [deploy.sh](/Users/luca/go/github.com/luca/video-saas/infra/aws/scripts/deploy.sh) logs into ECR, pulls the new images, and runs `docker compose up -d`.
6. The PostgreSQL container is published on `POSTGRES_PUBLIC_PORT` so your local machine can connect directly. Do not open that port to the world; restrict the EC2 security group to your own office or home IP.
7. [validate.sh](/Users/luca/go/github.com/luca/video-saas/infra/aws/scripts/validate.sh) checks the running stack.

## Notes

- TLS is handled by Caddy inside Docker. As long as DNS already points to the instance and ports `80/443` are reachable, certificates are issued automatically.
- This is the cheapest starter topology, not the final scale topology.
- When traffic grows, the migration path is:
  - move PostgreSQL out of the box first
  - move Redis / RabbitMQ out next
  - then split into ECS services behind ALB
