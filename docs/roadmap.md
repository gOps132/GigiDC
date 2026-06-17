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

Current status: gateway adapter, `/ping` slash handler, opt-in slash publishing, DM routing, guild-mention routing, capability evaluator, identity resolver contract, DB-backed role-first `/permissions` command, guild-scoped `/llm` provider and model controls, guild mention chat fallback, startup migration runner, and durable audit-log seam are started. Rich DM conversation, retrieval, assignment/task commands, and restricted action execution remain.

## v0 Memory And Actions Slice

- multi-owner LLM provider registry foundation
- guild-scoped provider credential UX
- model profiles for chat, reasoning, embedding, and routing
- durable jobs
- message history
- semantic retrieval
- tasks
- relays with confirmation
- usage tracking

Current LLM direction: build the data model and resolver for `guild`, `user`, and `tenant` credential owners from the start, but expose only guild/admin-scoped provider configuration in v0. Personal BYOK remains disabled in v0 product behavior. See [LLM And Cognitive Layer Plan](./llm-cognitive-plan).

## v1 Personal BYOK Slice

- optional user-owned provider credentials
- personal-key usage in DMs without guild memory or guild actions
- guild policy for personal keys: `off`, `dm-only`, or `guild-allowed`
- explicit billing-owner choice when a guild permits personal keys
- no automatic fallback between guild, user, or tenant billing owners
- personal keys pay for reasoning but never grant Gigi capabilities

## v0 External App Integration Slice

- approved external app catalog
- guild enable/configure flow
- external app declared prefix dry-run matching
- opt-in public `send_message` prefix dispatch
- external app permissions and audit logs
- external app behavior through approved manifests

Current status: manifest validation, exact Discord application/bot identity lookup, manifest source metadata, approved-manifest storage, enabled-guild manifest loading, Discord `/plugins` admin commands, deterministic guild mention dry-run matching, semantic dry-run routing, and public `send_message` prefix dispatch are started. Command publishing, restricted dispatch, confirmed per-message approval, and richer external app execution remain future slices. Any domain behavior only exists if an approved installed external app manifest declares it and the external app supports it.
