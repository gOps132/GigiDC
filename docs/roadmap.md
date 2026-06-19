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
- external app integration, job, Discord, storage, LLM, memory, and agent-runtime interfaces

## v0 Discord Surface Slice

- Discord gateway adapter behind `GIGI_DISCORD_ENABLED`
- slash command registration
- guild mention handling
- DM handling
- permission model

Current status: gateway adapter, `/ping` slash handler, opt-in slash publishing, DM routing, guild-mention routing, `/ask`, capability evaluator, identity resolver contract, DB-backed role-first `/permissions` command, guild-scoped `/llm` provider and model controls, agent-backed guild mention chat fallback, startup migration runner, guild memory settings/status/count/search scaffold, dynamic current-channel context fetching, semantic memory/tool routing policy, and durable audit-log seam are started. Rich DM conversation, semantic retrieval, assignment/task commands, and restricted action execution remain.

## v0 Agent Runtime Core Slice

- multi-owner LLM provider registry foundation
- guild-scoped provider credential UX
- model profiles for chat, reasoning, embedding, and routing
- agent handler behind deterministic Discord/plugin routing
- typed planner contract for intents, context requests, tool calls, clarification, and confirmation
- context broker for current-channel context, permitted guild memory, and enabled plugin catalog context
- native tool registry for deterministic capabilities such as `memory.count`, `memory.search`, `memory.recent`, `plugins.plan`, and `llm.chat`
- policy layer for capability checks, channel visibility, confirmation, redaction, and audit classification
- token-budgeted context packing with citations/tool result summaries
- durable jobs
- opt-in guild memory policy
- async message ingestion for enabled channels
- deterministic memory count queries
- semantic retrieval with citations
- tasks
- relays with confirmation
- usage tracking

Current agent direction: Gigi should become a Discord agent runtime, not a chatbot plus scattered plugins. Natural language should map to deterministic tools through an LLM planner. Gigi validates, checks permissions, enforces channel visibility, executes tools in Go, composes answers, and audits the path. This prevents writing one parser for every phrasing while keeping exact tools such as `memory.count` and `memory.search` as sources of truth.

Agent runtime work now includes dynamic context fetching for `/ask context:channel` and guild mentions: the planner can receive current-channel memory context, the broker fetches only permitted retained messages, the packer trims snippets deterministically, and context/tool/answer steps emit redacted traces.

Current LLM direction: build the data model and resolver for `guild`, `user`, and `tenant` credential owners from the start, but expose only guild/admin-scoped provider configuration in v0. Personal BYOK remains disabled in v0 product behavior. See [LLM And Cognitive Layer Plan](./llm-cognitive-plan).

Current memory direction: memory starts off, is enabled per channel, ingests through a bounded async live queue, and exposes deterministic tools for exact counts and search. Questions such as `@Gigi how many times did @sam mention "postgres"?` or `@Gigi how often do we talk about postgres here?` can use the planner to choose `memory.count`, but SQL computes the count over indexed, permitted messages. Embeddings are batched in workers and use the guild `embedding` profile, never personal BYOK for shared guild memory.

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
