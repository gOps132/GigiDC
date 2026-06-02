---
title: Setup
description: Local setup for the Go foundation runtime.
---

# Setup

## Requirements

- Go 1.26.3
- Docker with Compose plugin

## Configure

```bash
cp .env.example .env
```

The default runtime only requires `GIGI_DATABASE_URL`.

Set `GIGI_DISCORD_ENABLED=true` only when `DISCORD_TOKEN` and `DISCORD_CLIENT_ID` are configured. Discord starts as a gateway connection first; slash commands, DMs, mentions, and plugin execution land in later slices.

Set `GIGI_DISCORD_SYNC_COMMANDS=true` only when you want Gigi to bulk-overwrite its current slash command set. Use `GIGI_DISCORD_GUILD_ID` for a test server during development; leaving it blank targets global application commands.

## Run Checks

```bash
go test ./...
go vet ./...
go build ./cmd/gigi
```

## Run With Docker Compose

```bash
docker compose -f compose.yaml up --build
```

Then verify:

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

## Database

Local PostgreSQL starts through Docker Compose. The initial schema lives under `db/migrations/` and is mounted into the Postgres container for fresh database creation.

The database is exposed on `127.0.0.1:55432` by default to avoid collisions with a local Postgres on `5432`. Override with `GIGI_DB_PORT` if needed.

Supabase is not used by the foundation runtime.
