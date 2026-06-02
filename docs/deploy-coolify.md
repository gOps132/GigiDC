---
title: Coolify Deployment
description: Soft-deploy the unfinished Go foundation through Coolify.
---

# Coolify Deployment

Use Coolify as the deploy target for the unfinished Go foundation.

## Source

- Repository: `github.com/gOps132/GigiDC`
- Branch: `main` for normal deploys
- Build source: root `Dockerfile`
- Runtime shape: Docker Compose app + PostgreSQL/pgvector

## Required Environment

Set these values in Coolify, not in git:

- `GIGI_ENV=production`
- `GIGI_HTTP_ADDR=:8080`
- `GIGI_DATABASE_URL=postgres://gigi:<password>@db:5432/gigi?sslmode=disable`
- `POSTGRES_DB=gigi`
- `POSTGRES_USER=gigi`
- `POSTGRES_PASSWORD=<secure-password>`

Keep these disabled for the first soft deploy:

- `GIGI_DISCORD_ENABLED=false`
- `GIGI_DISCORD_SYNC_COMMANDS=false`

Only enable Discord after `/healthz` and `/readyz` pass.

## Health Checks

Use:

```text
/readyz
```

Expected response:

```json
{"ok":true}
```

## Current Limits

This deploy proves container, database, and health/readiness wiring only. It does not provide DM chat, mention chat, permission enforcement, audit logs, LLM calls, or plugin execution yet.
