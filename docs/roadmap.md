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

## v0 Discord Surface Slice

- Discord gateway adapter behind `GIGI_DISCORD_ENABLED`
- slash command registration
- guild mention handling
- DM handling
- permission model

Current status: gateway adapter, `/ping` slash handler, opt-in slash publishing, DM routing, guild-mention routing, capability evaluator, identity resolver contract, DB-backed role-first `/permissions` command, startup migration runner, and durable audit-log seam are started. Rich conversation, usage/ingestion/assignment/task commands, and action execution remain.

## v0 Memory And Actions Slice

- durable jobs
- message history
- semantic retrieval
- tasks
- relays with confirmation
- usage tracking

## v0 Plugin Skills Slice

- approved plugin catalog
- guild enable/configure flow
- plugin-declared prefix commands
- plugin permissions and audit logs
- plugin-specific behavior through approved manifests

Current status: manifest validation, exact Discord application/bot identity lookup, manifest source metadata, approved-manifest storage, enabled-guild manifest loading, and Discord `/plugins` admin commands are started. Command publishing, prefix routing, and plugin execution remain future slices. Any domain behavior only exists if an approved installed plugin declares and implements it.
