---
title: Coolify Deployment
description: Soft-deploy the unfinished Go foundation through Coolify.
---

# Coolify Deployment

Use Coolify as the deploy target for the unfinished Go foundation.

## Source

- Repository: `github.com/gOps132/GigiDC`
- Branch: `main` for normal deploys
- Build pack: Docker Compose
- Base directory: `/`
- Docker Compose location: `/docker-compose.yml`
- Runtime shape: app + PostgreSQL/pgvector
- Service port: `8080`

## Required Environment

Set these values in Coolify, not in git. You can copy the same block from [.env.coolify.example](/Users/giancedrick/dev/projects/gigi/.env.coolify.example).

```env
POSTGRES_DB=gigi
POSTGRES_USER=gigi
POSTGRES_PASSWORD=<secure-password>

GIGI_ENV=production
GIGI_DISCORD_ENABLED=false
GIGI_DISCORD_SYNC_COMMANDS=false
GIGI_DISCORD_GUILD_ID=
DISCORD_TOKEN=
DISCORD_CLIENT_ID=

OPENAI_API_KEY=
```

`docker-compose.yml` marks `POSTGRES_PASSWORD` as required with `${POSTGRES_PASSWORD:?}` so Coolify can surface it in environment setup.

Do not paste `docker compose config` output into issues, PRs, or chat after real secrets are set; Compose expands environment values.

Only enable Discord after `/healthz` and `/readyz` pass. With Discord enabled, the current safe smoke test is `/ping`, DM `ping`, or `@Gigi ping`.

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

This deploy proves container, database, health/readiness wiring, Discord gateway login, slash command sync, and basic DM/mention routing. Capability and audit foundations exist for future privileged actions. It does not provide rich DM chat, rich mention chat, admin grant commands, LLM calls, or plugin execution yet.
