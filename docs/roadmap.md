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
- external app integration, job, Discord, storage, and LLM interfaces

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

## v0 External App Integration Slice

- approved external app catalog
- guild enable/configure flow
- external app declared prefix dry-run matching
- external app permissions and audit logs
- external app behavior through approved manifests

Current status: manifest validation, exact Discord application/bot identity lookup, manifest source metadata, approved-manifest storage, enabled-guild manifest loading, Discord `/plugins` admin commands, and deterministic guild mention dry-run matching are started. Command publishing, confirmed dispatch, and external app command execution remain future slices. Any domain behavior only exists if an approved installed external app manifest declares it and the external app supports it.
