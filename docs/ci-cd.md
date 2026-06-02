---
title: CI/CD
description: Continuous integration and Coolify deployment direction for GigiDC.
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

The application image is built from the root `Dockerfile`. Build metadata can be injected with Go linker flags:

- version
- commit
- build time

## Deploy

Deployment is handled by Coolify or another Docker host that builds this repository and runs Docker Compose.

This repo does not ship a host-specific release uploader, service unit, or reverse-proxy config. Pushes to `main` only run CI validation.

## Smoke Checks

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
docker compose -f compose.yaml ps
```
