---
title: CI/CD
description: Continuous integration and Docker Compose deployment workflow for GigiDC.
---

# CI/CD Guide

## CI

GitHub Actions runs:

- `gofmt` check
- `go vet ./...`
- `go test ./...`
- `go build ./cmd/gigi`
- Docker Compose smoke test against `/healthz` and `/readyz`

Workflow file:

- [.github/workflows/ci.yml](/Users/giancedrick/dev/projects/gigi/.github/workflows/ci.yml)

## Build

The production image is built from the root `Dockerfile`. Build metadata is injected with Go linker flags:

- version
- commit
- build time

## Deploy

On pushes to `main`, CI packages:

- Docker image tarball
- `compose.prod.yaml`
- database migration SQL files
- `scripts/install-release.sh`

The deploy job uploads those files to EC2, runs the install script, loads the image, and starts the stack with Docker Compose. No infrastructure provisioning tool is required by this repo.

## Required GitHub Secrets

- `DEPLOY_HOST`
- `DEPLOY_SSH_KEY`

Optional:

- `DEPLOY_USER` default `ubuntu`
- `DEPLOY_PORT` default `22`

## Server Requirements

The host needs:

- Docker Engine
- Docker Compose plugin
- Nginx if exposing health checks publicly
- `/etc/gigi-discord-bot/gigi-discord-bot.env`
- `/opt/gigi-discord-bot`

Run the bootstrap script once:

```bash
sudo bash scripts/bootstrap-ec2.sh
```

## Smoke Checks

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
docker compose -f /opt/gigi-discord-bot/compose.yaml ps
```
