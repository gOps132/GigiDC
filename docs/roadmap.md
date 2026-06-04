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

Current status: gateway adapter, `/ping` slash handler, opt-in slash publishing, DM routing, guild-mention routing, capability evaluator, identity resolver contract, DB-backed role-first `/permissions` command, startup migration runner, and durable audit-log seam are started. Rich conversation, usage/ingestion/assignment/task commands, and action execution remain.

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
- plugin-declared prefix commands
- plugin permissions and audit logs
- plugin-specific behavior through approved manifests

Current status: manifest validation, exact Discord application/bot identity lookup, manifest URL metadata, approved-manifest storage, and enabled-guild manifest loading are started. Discord `/plugins` admin commands, guild enable/configure flow, command publishing, prefix routing, and plugin execution remain future slices. Any domain behavior only exists if an approved installed plugin declares and implements it.
