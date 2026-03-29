# V1 Architecture

## Summary

V1 is a reduced Discord bot architecture built for:

- DM-first agentic interaction
- participant-visible shared Gigi tasks and actions for relay and follow-up continuity
- bounded multi-tool DM execution on top of that shared task/action substrate
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
- autonomous memory promotion from arbitrary chat
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
  - routes explicit tool-style DM requests through a bounded planner/executor path before retrieval
  - decides whether a DM question needs a scope picker
  - persists pending scope-selection state so Discord select menus survive process restarts
  - resolves `This DM` vs guild-wide history access
  - persists bot-authored DM prompts and answers immediately so canonical history does not depend on gateway echo timing
  - calls retrieval and sends the final Discord reply
- `AgentToolService`
  - turns explicit DM requests into up to three internal tool calls
  - enforces capability checks and participant access before executing task or relay operations
  - rejects Gigi-mediated DM relays unless the recipient also has explicit receive permission
  - resolves Discord users conservatively from self references, mentions, or exact guild names
  - records audit events for tool execution, denial, and failure paths
- `AgentActionService`
  - persists participant-visible Gigi actions such as DM relays and follow-up tasks
  - records requester, assignee/recipient, status, due date, and delivery metadata
  - gives retrieval a durable shared-identity seam without exposing raw guild history broadly
- `RetrievalService`
  - routes phrase-count questions to exact SQL/RPC paths
  - falls back to recent-message context plus semantic search
  - adds participant-visible agent action and open-task context for history-aware follow-up questions
  - asks OpenAI Responses for the final natural-language answer
- `MessageHistoryService`
  - stores DM history immediately and stores guild history only for channels with ingestion enabled
  - explicitly stores outbound bot-authored DMs so Gigi's own replies and relay deliveries become canonical history
  - queues message embeddings for background indexing instead of generating them on the Discord gateway hot path
  - serves recent-history, phrase-count, and semantic-search queries
- `AssignmentService`
  - creates, lists, and publishes assignment notices
- `AuditLogService`
  - persists operational audit rows for policy changes and security-relevant command outcomes
- `RolePolicyService`
  - upserts guild rows and checks capability-based access

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

The latest extension of that seam is `ToolPlanningClient`, which keeps the DM tool planner behind a port instead of binding service code directly to OpenAI structured-output details.

### Data Schema

The graph shows two distinct persistence concerns.

Control-plane schema:

- `guilds`
- `role_policies`
- `assignments`
- `agent_actions`
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
- bounded task and relay execution when the request is explicit enough to plan safely

If multiple scopes are possible and allowed, the bot asks the user to choose via a Discord select menu.

The DM flow is effectively:

```text
Discord DM
  -> message stored
  -> optional tool planner + internal tool execution
  -> optional persisted scope selection
  -> retrieval strategy selection
  -> participant-visible agent action lookup
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

### DM Tool Runtime

The new DM tool runtime is intentionally narrow.

It currently supports:

- create follow-up task
- list open tasks
- complete task
- send DM relay

It does not support:

- web browsing or web search
- code execution or shell access
- image generation
- arbitrary external tools

The runtime shape is:

```text
Discord DM
  -> DmConversationService
  -> AgentToolService
  -> ToolPlanningClient
  -> task / relay execution
  -> agent_actions + audit_logs + messages
  -> deterministic DM reply
```

This means Gigi can now do limited multi-step work in DM, such as:

- create a task and then list open tasks in the same turn
- complete a task and confirm it
- send a participant-visible DM relay without switching to slash commands

This is still not a general tool framework. It is a bounded internal action runtime attached to the shared task/action substrate.

### Shared Identity Foundation

GigiDC now has a narrow but real shared-identity seam.

The current foundation is not "remember everything and answer everything." It is:

- one Gigi identity across guild commands and DMs
- durable `agent_actions` records for explicit Gigi-mediated actions and follow-up tasks
- participant-scoped visibility so requesters and recipients can ask follow-up questions later
- explicit persistence of Gigi's own DM outputs in `messages` so recall can use raw history as well as action summaries
- retrieval that can answer from those task and action records even when the user does not have guild-wide history access

The current concrete workflows are:

- `/relay dm`, which creates an `agent_actions` record, sends the DM, records success or failure, and lets the participants ask Gigi about that relay later in DM
- `/task create`, `/task list`, and `/task complete`, which turn `agent_actions` into a broader task/action substrate instead of a relay-only memory seam
- DM tool execution, which lets explicit natural-language DM requests create tasks, list tasks, complete tasks, or send relays without leaving the DM surface

### Shared Identity Tradeoffs

This architectural change improves continuity, but it also introduces real costs and failure modes:

- **History growth and context rot**
  - storing more bot-authored DM outputs makes the history corpus grow faster
  - if retrieval keeps pulling larger or lower-signal windows, answer quality can degrade because stale or repetitive context competes with the relevant message or action
  - this is one reason V2 still needs digesting, ranking improvements, and tighter retrieval windows instead of just "more memory"
- **Higher storage and embedding cost**
  - every persisted bot-authored message increases `messages` volume
  - if those messages are embedded, OpenAI embedding usage and Supabase storage both rise with usage
  - this is acceptable for the current scope, but it does not scale indefinitely without retention policy, pruning, or summarization
- **Dual-memory representations**
  - a successful relay now exists both as an `agent_actions` row and as raw DM history in `messages`
  - that improves traceability, but it also creates duplication and possible ranking ambiguity
  - follow-up tasks now share the same substrate, which increases the need for retrieval to distinguish between conversational context, action history, and open work
  - retrieval has to avoid double-counting the same event or over-weighting relays or tasks compared with ordinary conversation
- **Privacy and retention implications**
  - more private bot-authored DM content is now canonical project data rather than transient output
  - that raises the bar for retention policy, deletion workflows, audit review, and least-privilege access to stored history
  - a future "shared memory" model must stay permission-aware so continuity does not become cross-user leakage
- **Operational asymmetry still exists**
  - outbound DM persistence is now explicit, which is better than relying on gateway echoes
  - but indexing is still an in-process queue, so a restart can still leave some recent history unembedded even when the raw message row exists
  - this means continuity is improved before durability is fully solved
- **Planner ambiguity and conservative execution**
  - the DM tool planner is deliberately narrow and conservative, which reduces risky actions but also means some valid user requests will fall back to retrieval instead of executing
  - user resolution for relays or assigned tasks can fail when the request uses nicknames, vague labels, or ambiguous names instead of a Discord mention or exact name
  - this is preferable to dispatching the wrong relay or completing the wrong task, but it does create user-friction that a richer identity or directory layer would later need to address
- **Extra model and latency cost**
  - tool-style DM requests now spend an additional model call on planning before any actual execution happens
  - that keeps execution safer and more structured, but it raises per-turn latency and API cost relative to direct retrieval-only turns
- **Still not durable orchestration**
  - tool execution happens synchronously during the DM turn
  - a process crash mid-turn can still leave the user without a final reply even though some writes may already have happened
  - there is still no durable worker, retry queue, or background execution model for longer-running or multi-step actions

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
- `/relay dm`
- `/task create`
- `/task list`
- `/task complete`
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
- agent_actions
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

Shared Gigi relay and task dispatch is role-gated by `agent_action_dispatch`.

Receiving a Gigi-mediated DM relay is role-gated by `agent_action_receive`.

Participant-visible action and task recall does not require guild-wide history access. Retrieval can use `agent_actions` for the requester or recipient/assignee of a Gigi-mediated relay or task while still keeping unrelated guild history gated.

DM tool execution uses the same permission boundaries:

- listing your own tasks is allowed without dispatch capability
- completing a task is allowed for visible participants
- creating shared tasks or sending relays still requires `agent_action_dispatch`
- sending a relay also requires the recipient to have `agent_action_receive`
- listing another user's tasks still requires `agent_action_dispatch`

## Current Risks

The current shared-identity foundation is intentionally narrow, but the repo should treat these as active architectural risks:

- retrieval quality can decay as DM and bot-authored history grows unless ranking and summarization improve
- canonical storage now contains more private conversational material, which increases retention and deletion pressure
- duplicated signal across `messages` and `agent_actions` can create noisy context assembly
- the lack of a durable indexing worker means semantic recall can lag behind raw-history recall after restarts
- broader shared-memory features should not be added by simply widening retrieval scope; they need explicit visibility rules and task boundaries
- the current DM tool planner can misclassify natural language or fail to resolve people unless the request is explicit
- unsupported-capability questions still need explicit grounding rules or the language model will invent broader tool access than the bot actually has
- multi-tool execution is still bounded to short synchronous internal actions and should not be mistaken for a general agent-worker system

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
- relay permission denials are logged
- relay success and failure outcomes are logged
- task creation and completion outcomes are logged

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
