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

Discord login is disabled by default through `GIGI_DISCORD_ENABLED=false`. When enabled, the gateway can publish `/ping`, `/permissions`, and `/plugins`, answer `/ping`, route DMs, route guild mentions, ignore ordinary unmentioned guild messages, and ignore bot-authored messages. External app manifest validation, storage, import, enable, disable, deterministic dry-run matching, and public `send_message` prefix dispatch have started, but restricted dispatch, retrieval, and LLM calls are intentionally not active yet.

Capability and identity foundations now exist as internal gates for privileged actions. Role and user capability grants are keyed by Discord IDs, not role names, and admin override is modeled separately from role grants.

## Target Shape

```text
Discord Gateway
  -> Go runtime
     -> interaction and DM router
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
- `internal/llm`: LLM client contracts for later slices.
- `internal/llm/provider` (planned): provider registry, encrypted credentials, model profiles, usage records, and credential resolution for OpenAI, Anthropic, Gemini, and future providers.
- `internal/assistant` or `internal/cognitive` (planned): surface-independent orchestration for rich chat, retrieval, action plans, and semantic routing.

## Data Boundary

Local PostgreSQL is the new source of truth. The first migration creates foundation tables for runtime metadata, external app installs, role/user capability grants, jobs, outbox events, and audit logs. The plugin catalog migration adds exact Discord application/bot identity lookup, manifest source metadata, approval metadata, and enabled-install indexes. The app also runs the idempotent migration files on startup so existing Docker volumes can catch up. Supabase is not part of the live runtime and no backfill is planned.

LLM provider storage should support multiple credential owners from the first schema: `guild`, `user`, and `tenant`. V0 should expose only guild/admin-scoped provider configuration, while leaving user-owned BYOK and tenant/operator fallback credentials as policy-controlled later behavior. Usage records should preserve billing owner type, billing owner ID, actor, provider, model, purpose, token counts, status, and classified error without storing raw prompts, completions, provider responses, or secrets.

## LLM And Cognitive Direction

Gigi should use a provider registry with first-class OpenAI, Anthropic, and Gemini entries plus room for future custom providers. Model profiles should be selected by purpose: `chat`, `reasoning`, `embedding`, and `routing`.

The cognitive layer should sit behind deterministic external app matching. Exact enabled plugin prefix triggers remain first; if none match, a cognitive fallback can handle rich DM or guild-mention conversation. LLM output must only propose drafts or plans. Gigi builds final action plans from stored manifests, capability checks, confirmation policy, and audit rules.

Personal BYOK should not be required for v0. A guild admin may provide a provider key, but once it powers shared server behavior, Gigi treats it as a guild credential governed by guild policy and audit. V1 can add optional user-owned keys for DMs and explicit guild-approved personal billing, but personal keys must not grant capabilities or silently process guild context.

## External App Direction

Gigi will understand approved external Discord apps and bots from manifests. During v0, discovery is exact-match only: a known manifest must match a Discord application ID or bot user ID, or an operator/admin must provide an approved HTTPS manifest URL or uploaded JSON manifest. A guild admin can enable an approved integration, then Gigi can match guild mention text against declared prefix triggers. Public actions use empty `permissions`; restricted actions still require capability checks. If the manifest explicitly declares `dispatch: "send_message"` and the matched action is public, Gigi sends the planned prefix command into the channel as Gigi. Later slices can handle restricted dispatch, slash commands, buttons, mentions, DMs, or natural-language requests to that external app after config checks.

## Known Limits

- No Discord gateway connection unless `GIGI_DISCORD_ENABLED=true`.
- No slash command publishing unless `GIGI_DISCORD_SYNC_COMMANDS=true`.
- DM routing only has `ping` plus placeholder replies. Guild mentions can also dry-run enabled external app prefix triggers or dispatch public `send_message` triggers.
- `/permissions` can create/assign Discord roles, grant/revoke role capabilities and presets, and manage direct user exceptions.
- `/plugins` can list approved manifests, import HTTPS manifests or uploaded JSON manifests, enable approved external app versions for a guild, disable guild integrations, and list enabled guild integrations.
- Durable audit store is used for permission checks and permission changes, but current Discord liveness replies do not depend on it yet.
- External app command dispatch is limited to public `send_message` prefix actions declared by approved enabled manifests. Restricted actions stay dry-run only.
- External apps may ignore bot-authored messages after Gigi sends the planned command.
- No LLM calls yet.
- No retrieval or memory behavior yet.
- Readiness checks database reachability; startup applies idempotent SQL migration files before Discord command wiring.
