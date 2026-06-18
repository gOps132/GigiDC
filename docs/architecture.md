---
title: Architecture
description: Current architecture model for the Go foundation rebuild.
---

# Architecture

## Current State

Gigi is in a foundation rebuild. The old Node/Supabase runtime is gone. The current runtime is a Go service that exposes:

- `GET /healthz`
- `GET /readyz`
- build metadata through `internal/buildinfo`
- config loading through `internal/config`
- database reachability through `internal/storage`

Discord login is disabled by default through `GIGI_DISCORD_ENABLED=false`. When enabled, the gateway can publish `/ping`, `/permissions`, `/llm`, and `/plugins`, answer `/ping`, route DMs, route guild mentions, ignore ordinary unmentioned guild messages, and ignore bot-authored messages. External app manifest validation, storage, import, enable, disable, deterministic dry-run matching, semantic dry-run routing, guild mention chat fallback, and public `send_message` prefix dispatch have started. Provider HTTP text calls are live when a guild credential, model profile, and `GIGI_LLM_SECRET_KEY_BASE64` are configured. Restricted dispatch, retrieval, memory, and rich DM chat remain unavailable.

Capability and identity foundations now exist as internal gates for privileged actions. Role and user capability grants are keyed by Discord IDs, not role names, and admin override is modeled separately from role grants.

## Target Shape

```text
Discord Gateway
  -> Go runtime
     -> interaction and DM router
     -> agent runtime core
        -> runner
        -> planner
        -> policy
        -> executor
        -> context broker
        -> native tool registry
        -> answer composer
        -> redacted trace
     -> capability engine
     -> external app integration catalog
     -> durable jobs and outbox
     -> PostgreSQL + pgvector
     -> LLM provider registry
     -> cognitive orchestration layer
```

## Package Boundaries

- `cmd/gigi`: binary entrypoint and process lifecycle.
- `internal/app`: application wiring.
- `internal/config`: environment parsing.
- `internal/web`: health and readiness HTTP handlers.
- `internal/storage`: database reachability, startup migration runner, and future storage seams.
- `internal/capability`: role/user capability evaluation plus grant/revoke management with guild owner and Discord administrator override.
- `internal/identity`: synchronous identity resolution contract for future privileged actions.
- `internal/audit`: audit event validation and durable audit-log store seam.
- `internal/plugins`: approved external app manifest validation, exact Discord identity catalog lookup, and SQL-backed enabled-manifest loading.
- `internal/jobs`: durable job contracts.
- `internal/discord`: Discord gateway adapter, slash command router, DM/guild-mention router, and audit seam.
- `internal/llm`: provider-backed text client contracts and HTTP callers for OpenAI, Anthropic, Gemini, and custom-compatible providers.
- `internal/llm/provider`: provider registry, encrypted credentials, model profiles, usage records, provider testing, and credential resolution for OpenAI, Anthropic, Gemini, and future providers.
- `internal/assistant`: current surface-independent orchestration for guild-mention chat, metadata-only conversation turns, native memory planning, and semantic plugin routing.
- `internal/agent`: agent runtime core for bounded runs, request planning, policy checks, tool execution, answer composition, confirmation, and audit-friendly traces.
- `internal/contextbroker`: planned scoped context retrieval layer for current-channel context, permitted guild memory, enabled plugin catalog summaries, and token-budgeted context packs.
- `internal/tools`: planned registry for native deterministic tools and enabled external app tool surfaces.

## Data Boundary

Local PostgreSQL is the new source of truth. The first migration creates foundation tables for runtime metadata, external app installs, role/user capability grants, jobs, outbox events, and audit logs. The plugin catalog migration adds exact Discord application/bot identity lookup, manifest source metadata, approval metadata, and enabled-install indexes. The app also runs the idempotent migration files on startup so existing Docker volumes can catch up. Supabase is not part of the live runtime and no backfill is planned.

LLM provider storage supports multiple credential owners from the first schema: `guild`, `user`, and `tenant`. V0 exposes only guild/admin-scoped provider configuration, model selection, credential tests, credential resolution, and aggregate guild usage. User-owned BYOK and tenant/operator fallback credentials remain policy-controlled later behavior. Usage records preserve billing owner type, billing owner ID, actor, provider, model, purpose, token counts, status, and classified error without storing raw prompts, completions, provider responses, or secrets.

## LLM And Cognitive Layer

Gigi uses a provider registry with first-class OpenAI, Anthropic, and Gemini entries plus room for custom providers. Model profiles are selected by purpose: `chat`, `reasoning`, `embedding`, and `routing`.

The cognitive layer is evolving into the agent runtime core. Exact deterministic handlers remain first. If `/llm routing` is `dry-run` or `enabled`, the routing model may propose typed native memory tools or manifest-grounded external app plans from registered tool schemas. Gigi validates every proposal through the runner policy, applies capability checks before tool execution, enforces channel visibility, and then either returns a dry-run plan or executes only allowed read-only/public-safe actions through the executor. Write-class tools are confirmation-required by default. Tool results can flow into an answerer that composes the final response, with short-term per-user/channel follow-up context for prompts such as "who said it?".

The core invariant is: LLM output is only a proposal. Gigi builds final action plans from registered native tool schemas, stored manifests, capability checks, confirmation policy, channel visibility rules, and audit rules. Deterministic tools such as `memory.count`, `memory.search`, `plugins.plan`, `permissions.check`, and usage summaries remain the source of truth. Natural language changes the path into those tools; it does not replace them.

The runner is bounded. V0 defaults keep runs short: a small step limit, a small tool-call limit, and a small LLM-call budget. Every step appends a redacted trace entry. Traces and audit metadata may include request IDs, run IDs, ordered step indexes, tool names, tool kind, status, capability, scope, and result counts, but not raw prompts, raw snippets, provider payloads, or secrets.

Personal BYOK should not be required for v0. A guild admin may provide a provider key, but once it powers shared server behavior, Gigi treats it as a guild credential governed by guild policy and audit. V1 can add optional user-owned keys for DMs and explicit guild-approved personal billing, but personal keys must not grant capabilities or silently process guild context.

## External App Direction

Gigi understands approved external Discord apps and bots from manifests. During v0, discovery is exact-match only: a known manifest must match a Discord application ID or bot user ID, or an operator/admin must provide an approved HTTPS manifest URL or uploaded JSON manifest. A guild admin can enable an approved integration, then Gigi can match guild mention text against declared prefix triggers. If deterministic matching fails and `/llm routing` permits it, semantic routing can propose a manifest-grounded plan. Public actions use empty `permissions`; restricted actions still require capability checks. If the manifest explicitly declares `dispatch: "send_message"` and the matched action is public, Gigi sends the planned prefix command into the channel as Gigi. Later slices can handle restricted dispatch, slash commands, buttons, DMs, richer natural-language execution, or direct webhook dispatch after config checks.

## Known Limits

- No Discord gateway connection unless `GIGI_DISCORD_ENABLED=true`.
- No slash command publishing unless `GIGI_DISCORD_SYNC_COMMANDS=true`.
- DM routing only has `ping` plus a response that rich chat needs a server LLM profile first. Guild mentions can dry-run enabled external app prefix triggers, dispatch public `send_message` triggers, propose semantic dry-run plans, or answer through a configured chat model.
- `/permissions` can create/assign Discord roles, grant/revoke role capabilities and presets, and manage direct user exceptions.
- `/plugins` can list approved manifests, import HTTPS manifests or uploaded JSON manifests, enable approved external app versions for a guild, disable guild integrations, and list enabled guild integrations.
- Durable audit store is used for permission checks and permission changes, but current Discord liveness replies do not depend on it yet.
- External app command dispatch is limited to public `send_message` prefix actions declared by approved enabled manifests. Restricted actions stay dry-run only.
- External apps may ignore bot-authored messages after Gigi sends the planned command.
- Provider text calls require sealed guild credentials, active model profiles, and `GIGI_LLM_SECRET_KEY_BASE64`; personal BYOK, rich semantic retrieval, and rich DM chat are not live.
- Guild memory is limited to opted-in current-channel count/search behavior; cross-channel semantic retrieval, citations, backfill UX, and export/delete workflows remain later behavior.
- Readiness checks database reachability; startup applies idempotent SQL migration files before Discord command wiring.
