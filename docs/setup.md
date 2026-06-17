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

The default runtime requires `GIGI_DATABASE_URL` and can use `GIGI_MIGRATIONS_DIR` when migrations are not under `db/migrations`.

Set `GIGI_DISCORD_ENABLED=true` only when `DISCORD_TOKEN` and `DISCORD_CLIENT_ID` are configured. Discord starts with `/ping`, DM/mention liveness routing, `/permissions`, `/plugins`, external app dry-run matching, and opt-in public `send_message` prefix dispatch; rich chat and restricted external app dispatch land in later slices.

Set `GIGI_DISCORD_SYNC_COMMANDS=true` only when you want Gigi to bulk-overwrite its current slash command set. Use `GIGI_DISCORD_GUILD_ID` for a test server during development; leaving it blank targets global application commands.

LLM provider add/rotate commands require `GIGI_LLM_SECRET_KEY_BASE64`; when blank, `/llm provider add` and `rotate` stay disabled while list/delete/model commands remain available. The key must be standard base64 for exactly 32 bytes. `GIGI_LLM_SECRET_KEY_ID` defaults to `local-v1`.

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

Local PostgreSQL starts through Docker Compose. The initial schema lives under `db/migrations/` and is mounted into the Postgres container for fresh database creation. The app also runs the same idempotent migration files on startup, which lets an existing Docker volume catch up after schema-only foundation changes.

The database is exposed on `127.0.0.1:55432` by default to avoid collisions with a local Postgres on `5432`. Override with `GIGI_DB_PORT` if needed.

Supabase is not used by the foundation runtime.
