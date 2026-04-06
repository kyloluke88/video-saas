# Environment Guide

This file defines the intended difference between local development, the current production deployment, and the later full production deployment with worker support.

## Overview

This repository currently has three meaningful environment shapes:

1. Local development
2. Production web-only
3. Production full stack in the future

The important point is that production is intentionally lighter than local right now. We are not mirroring every local service to the cloud yet.

## Local Development

Local development is the full feature environment.

Services:

- `frontend`
- `backend`
- `postgres`
- `redis`
- `rabbitmq`
- `worker`

Primary file:

- [docker-compose.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.yml)

Key behavior:

- `backend` expects RabbitMQ to be enabled.
- `worker` is present and consumes tasks.
- podcast / video generation can run locally.
- local assets and outputs are written inside the repo bind mounts.

Typical env shape:

- [backend/.env](/Users/luca/go/github.com/luca/video-saas/backend/.env)
- [worker/.env](/Users/luca/go/github.com/luca/video-saas/worker/.env)

Important local setting:

- `RABBITMQ_ENABLED=true`

## Current Production

Current production is the lightweight public website environment.

Services:

- `caddy`
- `frontend`
- `backend`
- `postgres`
- `redis`

Not deployed right now:

- `rabbitmq`
- `worker`

Primary file:

- [docker-compose.prod.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.prod.yml)

Key behavior:

- `backend` starts with `RABBITMQ_ENABLED=false`.
- public script pages, public project pages, and normal frontend/backend reads work.
- any endpoint that tries to enqueue a task returns `503` because RabbitMQ is disabled.
- PostgreSQL is exposed on `POSTGRES_PUBLIC_PORT` so your local machine can connect directly.

Current production env files:

- [infra/aws/env/global.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/global.env.example)
- [infra/aws/env/backend.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/backend.env.example)
- [infra/aws/env/frontend.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/frontend.env.example)

Important production settings:

- `ENABLE_WORKER_STACK=false`
- `RABBITMQ_ENABLED=false`
- `POSTGRES_PUBLIC_PORT=15432` or another non-default public port

Database access from your local machine:

- host: your EC2 public IP or domain
- port: `POSTGRES_PUBLIC_PORT`
- database credentials: the values in `backend.env`

Security requirement:

- never open the PostgreSQL port to `0.0.0.0/0`
- in the EC2 security group, allow the PostgreSQL port only from your own office or home public IP

## Future Production Full Stack

Later, production can be upgraded to include the queue and worker without rebuilding the whole deployment model.

Additional services:

- `rabbitmq`
- `worker`

Additional file:

- [docker-compose.worker.prod.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.worker.prod.yml)

Additional env file:

- [infra/aws/env/worker.env.example](/Users/luca/go/github.com/luca/video-saas/infra/aws/env/worker.env.example)

Required flags for that future state:

- `ENABLE_WORKER_STACK=true`
- `RABBITMQ_ENABLED=true`
- `BUILD_WORKER_IMAGE=true` in CodeBuild

What changes when you enable the full stack:

- CodeDeploy starts both base compose and worker compose.
- RabbitMQ runs on the EC2 host.
- Worker runs on the EC2 host.
- Backend waits for RabbitMQ again.
- task submission endpoints become usable online.

## Deployment Files

Local:

- [docker-compose.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.yml)

Production base:

- [docker-compose.prod.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.prod.yml)

Production bootstrap before ECR:

- [docker-compose.bootstrap.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.bootstrap.yml)

Production worker extension:

- [docker-compose.worker.prod.yml](/Users/luca/go/github.com/luca/video-saas/docker-compose.worker.prod.yml)

AWS deployment helpers:

- [buildspec.yml](/Users/luca/go/github.com/luca/video-saas/buildspec.yml)
- [appspec.yml](/Users/luca/go/github.com/luca/video-saas/appspec.yml)
- [infra/aws/scripts/deploy.sh](/Users/luca/go/github.com/luca/video-saas/infra/aws/scripts/deploy.sh)
- [infra/aws/scripts/bootstrap_start.sh](/Users/luca/go/github.com/luca/video-saas/infra/aws/scripts/bootstrap_start.sh)

## Current Recommended Workflow

Right now the recommended workflow is:

1. Run generation locally.
2. Keep production focused on public page delivery.
3. Connect your local tooling to the production PostgreSQL instance through the published PostgreSQL port.
4. Restrict that PostgreSQL port in AWS security groups to your own IP only.
5. Move `worker` online later when the public site is stable.

## Why This Split Exists

This split is intentional.

- It lowers cloud cost.
- It lowers operational risk for the first public release.
- It avoids paying to run heavy worker infrastructure before the public site needs it.
- It keeps the migration path open for a later full cloud worker deployment.
