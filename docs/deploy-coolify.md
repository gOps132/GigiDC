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

Set these values in Coolify, not in git. You can copy the same block from `.env.coolify.example`.

```env
POSTGRES_DB=gigi
POSTGRES_USER=gigi
POSTGRES_PASSWORD=<secure-password>

GIGI_ENV=production
GIGI_MIGRATIONS_DIR=/app/db/migrations
GIGI_DISCORD_ENABLED=false
GIGI_DISCORD_SYNC_COMMANDS=false
GIGI_DISCORD_GUILD_ID=
DISCORD_TOKEN=
DISCORD_CLIENT_ID=

GIGI_LLM_SECRET_KEY_BASE64=
GIGI_LLM_SECRET_KEY_ID=local-v1
```

`docker-compose.yml` marks `POSTGRES_PASSWORD` as required with `${POSTGRES_PASSWORD:?}` so Coolify can surface it in environment setup.

Do not paste `docker compose config` output into issues, PRs, or chat after real secrets are set; Compose expands environment values.

Only enable Discord after `/healthz` and `/readyz` pass. With Discord enabled, the current safe smoke test is `/ping`, DM `ping`, `@Gigi ping`, `/permissions role grant` against a test role in a private admin channel, `/llm provider list`, or `/plugins list`.

Role create, assign, and unassign require the bot's Discord server role to have `Manage Roles`. For assign/unassign, the bot role must also sit higher than the target role. Channel membership or channel admin status is not enough because Discord enforces role changes at the server role level.

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

This deploy proves container, database, startup migrations, health/readiness wiring, Discord gateway login, slash command sync, basic DM/mention routing, admin-gated capability grants, guild-scoped LLM provider controls, aggregate LLM usage reporting, plugin catalog controls, external app dry-run matching, semantic external app dry-run routing, guild mention chat fallback when configured, and opt-in public `send_message` prefix dispatch. It does not provide retrieval, memory, rich DM chat, reasoning chat, assignment/task commands, or restricted external app dispatch yet.
