# V1 Architecture

## Summary

V1 is a reduced Discord bot architecture built for:

- DM-first agentic interaction
- slash-command assignment notices
- exact + semantic retrieval over raw Discord history
- role-gated guild-wide history access

The current repo now breaks down cleanly into six graph-derived layers:

- Runtime Entrypoints
- Application Services
- Platform Integrations
- Data Schema
- Delivery & Operations
- Project Docs

That layering matches the current codebase more closely than the original one-line runtime sketch and is the current baseline for `Understand-Anything` visualization work.

V1 intentionally still excludes:

- smart digestion
- memory promotion
- OCR / vision
- browser workers
- code sandbox workers

## Runtime Shape

```text
Discord
  -> Runtime Entrypoints
     -> Application Services
        -> Ports
           -> Platform Adapters
           -> Supabase / Postgres
           -> OpenAI
```

Detailed layout:

```text
src/index.ts
  -> config/env.ts
  -> lib/supabase.ts
  -> lib/openai.ts
  -> adapters/*
  -> ports/*
  -> services/*
  -> discord/registerCommands.ts
  -> discord/client.ts
  -> web/server.ts

discord/client.ts
  -> slash commands in src/commands/*
  -> DM message handling
  -> message history indexing
  -> DM retrieval orchestration
```

## Current Layer Responsibilities

### Runtime Entrypoints

This layer is the Discord-facing shell:

- `src/index.ts` bootstraps env, clients, services, command registration, and `/healthz`
- `src/discord/client.ts` owns gateway event handling for interactions and DMs
- `src/discord/registerCommands.ts` pushes slash-command definitions to Discord
- `src/commands/*` expose the current slash surface
- `src/web/server.ts` exposes the health endpoint used by infrastructure checks

### Application Services

This layer holds the actual bot behavior:

- `DmConversationService`
  - decides whether a DM question needs a scope picker
  - persists pending scope-selection state so Discord select menus survive process restarts
  - resolves `This DM` vs guild-wide history access
  - calls retrieval and sends the final Discord reply
- `RetrievalService`
  - routes phrase-count questions to exact SQL/RPC paths
  - falls back to recent-message context plus semantic search
  - asks OpenAI Responses for the final natural-language answer
- `MessageHistoryService`
  - stores DM history immediately and stores guild history only for channels with ingestion enabled
  - queues message embeddings for background indexing instead of generating them on the Discord gateway hot path
  - serves recent-history, phrase-count, and semantic-search queries
- `AssignmentService`
  - creates, lists, and publishes assignment notices
- `RolePolicyService`
  - upserts guild rows and checks capability-based access
- `AuditLogService`
  - persists operational audit rows for policy changes and security-relevant command outcomes

### Platform Integrations

This layer is intentionally thin and now split into explicit ports plus vendor adapters:

- `config/env.ts` validates runtime configuration
- `lib/supabase.ts` creates the admin Supabase client
- `lib/openai.ts` creates the OpenAI client
- `lib/logger.ts` and `lib/http.ts` provide shared runtime helpers
- `src/ports/*` define application-facing contracts for storage and AI access
- `src/adapters/*` implement those contracts with Supabase and OpenAI

### Ports And Adapters Boundary

The repo now follows a lightweight ports-and-adapters split inside the application layer:

- services own the bot's behavior and orchestration
- ports define what the services need from storage and model providers
- adapters translate those ports into Supabase queries or OpenAI SDK calls

This is the main architectural improvement from the current upgrade slice because it gives the bot cleaner seams for provider changes, fake-backed tests, and future worker extraction.

### Data Schema

The graph shows two distinct persistence concerns.

Control-plane schema:

- `guilds`
- `role_policies`
- `assignments`
- `audit_logs`
- `pending_dm_scope_selections`

Retrieval schema:

- `messages`
- `message_attachments`
- `message_embeddings`
- retrieval RPC/function paths used for phrase counts and embedding search

This split is important because the retrieval path is append-heavy and query-heavy, while the control-plane path is permission- and workflow-oriented.

### Delivery & Operations

Infrastructure and ops are a larger part of the repo than the old architecture note implied:

- `terraform/*` provisions the EC2 host, security group, and bootstrap wiring
- `scripts/bootstrap-ec2.sh`, `scripts/install-release.sh`, and `scripts/deploy-discord-bot.sh` handle server setup and deploy operations
- CI/CD documentation and release flow docs live under `docs/ci-cd.md` and `docs/deploy-ec2.md`

## Interaction Modes

### DMs

DMs are the primary agent interface.

Use them for:

- flexible questions
- semantic history lookup
- exact analytics like phrase counts
- scoped follow-up questions

If multiple scopes are possible and allowed, the bot asks the user to choose via a Discord select menu.

The DM flow is effectively:

```text
Discord DM
  -> message stored
  -> optional persisted scope selection
  -> retrieval strategy selection
  -> background embedding queue
  -> OpenAI answer synthesis
  -> DM reply
```

Guild ingestion is different:

```text
Discord Guild Message
  -> role/guild initialization
  -> channel_ingestion_policies check
  -> store only when explicitly enabled
  -> background embedding queue
```

This keeps DM interactions available by default while making guild-history retention an explicit policy choice.

### Background Indexing

Message embeddings are no longer generated inline on the Discord gateway path.

The current indexing path is:

```text
message stored
  -> MessageIndexingService enqueue
  -> OpenAI embeddings request
  -> message_embeddings upsert
```

This is still an in-process queue, not a durable worker system, but it removes the worst latency coupling from the runtime hot path.

### Slash Commands

Slash commands are for structured workflows:

- `/ping`
- `/ingestion enable`
- `/ingestion disable`
- `/ingestion status`
- `/assignment create`
- `/assignment publish`
- `/assignment list`

At startup the bot can register commands through the Discord REST API before it logs in to the gateway.

That behavior is controlled by `REGISTER_COMMANDS_ON_STARTUP`, which keeps boot and command registration separable for safer deploys and debugging.

## Data Split

### Control-plane tables

- guilds
- role_policies
- assignments
- audit_logs
- pending_dm_scope_selections

### Retrieval tables

- messages
- message_attachments
- message_embeddings

## Scope Rules

V1 supports:

- `This DM`
- `Guild-wide` in the configured primary guild

Guild-wide history access is role-gated by `history_guild_wide`.

Channel-ingestion administration is role-gated by `ingestion_admin`.

## Runtime Health

Runtime health is now two-tiered:

- `/healthz` returns process and runtime state for debugging
- `/readyz` returns readiness based on Discord connectivity, command-registration state, and indexing runtime status

This is a stronger operational boundary than the earlier always-`ok` health response.

## Audit Trail

The current audit surface is still selective, but ingestion-policy changes now form an explicit audit boundary:

- permission-denied ingestion admin attempts are logged
- ingestion enable and disable actions are logged
- no-op enable or disable attempts are also logged so admin intent is visible

This gives the repo a real operational trail around retention policy changes, which are more sensitive than ordinary read-only commands.

## Assignment Notifier

Assignment support in V1 is a notice workflow, not a full assignment management system.

Each notice stores:

- title
- description
- due date
- target channel
- affected role IDs
- creator
- publication status

Publishing posts a formatted notice to the target channel and mentions the affected roles.

## Deployment Shape

The current deployment model is:

```text
Terraform
  -> EC2 instance + security group
  -> bootstrap user-data
  -> release/deploy shell scripts
  -> systemd + Nginx host configuration
  -> running Discord bot process + /healthz + /readyz
```

## Visualization Baseline

The repo now keeps a generated knowledge graph under `.understand-anything/` for ongoing architecture analysis.

Use that graph and the dashboard as the fast path for:

- onboarding into the current runtime layout
- checking which services touch which schema objects
- refreshing docs and diagrams after cross-cutting changes
