---
title: Roadmap
description: Planned evolution of the Go foundation rebuild.
---

# Roadmap

## Foundation

- Go runtime
- Docker Compose
- PostgreSQL + pgvector
- health and readiness endpoints
- plugin, job, Discord, storage, and LLM interfaces

## V1 Discord Surface

- Discord gateway adapter behind `GIGI_DISCORD_ENABLED`
- slash command registration
- guild mention handling
- DM handling
- permission model

Current status: gateway adapter and `/ping` slash handler started; slash publishing, mention handling, DM handling, permission model, and audit remain.

## V2 Memory And Actions

- durable jobs
- message history
- semantic retrieval
- tasks
- relays with confirmation
- usage tracking

## V3 Plugin Skills

- approved plugin catalog
- guild enable/configure flow
- prefix commands such as `!play`
- plugin permissions and audit logs
- media/music plugin support as first real plugin candidate
