---
title: System Overview
description: High-level view of the current GigiDC system layers and flows.
---

# System Overview

This overview has been refreshed from the first real `Understand-Anything` graph pass. It reflects the current repo shape rather than the earlier placeholder runtime sketch.

```mermaid
flowchart LR
  Discord["Discord"]
  Register["Command Registration<br/>src/discord/registerCommands.ts"]
  Gateway["Gateway Runtime<br/>src/discord/client.ts"]
  Commands["Slash Commands<br/>src/commands/*"]
  DMFlow["DM Scope + Tools + Retrieval<br/>DmConversationService"]
  ToolRuntime["DM Tool Runtime<br/>AgentToolService + ToolPlanningClient"]
  Retrieval["RetrievalService"]
  History["Message Storage + Background Indexing<br/>MessageHistoryService + MessageIndexingService"]
  SharedIdentity["Shared Gigi Identity<br/>AgentActionService + agent_actions"]
  Admin["Assignment + Ingestion + Relay + Task Admin<br/>Role + Audit Services"]
  Ports["Ports<br/>history + control-plane + AI contracts"]
  Adapters["Platform Adapters<br/>Supabase + OpenAI"]
  DB["Supabase / Postgres"]
  Schema["Schema Split<br/>control-plane + retrieval"]
  Ops["Terraform + Deploy Scripts + CI/CD"]
  Health["GET /healthz + /readyz"]

  Discord --> Register
  Discord --> Gateway
  Gateway --> Commands
  Gateway --> DMFlow
  DMFlow --> ToolRuntime
  DMFlow --> Retrieval
  Gateway --> History
  ToolRuntime --> SharedIdentity
  Retrieval --> SharedIdentity
  Commands --> Admin
  Admin --> SharedIdentity
  Admin --> Ports
  ToolRuntime --> Ports
  Retrieval --> Ports
  History --> Ports
  Ports --> Adapters
  Adapters --> DB
  Adapters --> OpenAI["OpenAI"]
  DB --> Schema
  Ops --> Gateway
  Ops --> Health
```

## Reading Guide

- The runtime entrypoints are small: startup, command registration, gateway handling, and health/readiness endpoints.
- Most behavior sits in services, especially the DM runtime, the bounded tool-execution path, the shared-identity task/action path, and the admin command path for assignments, ingestion, relay dispatch, and task tracking.
- The service layer now depends on explicit ports, with Supabase and OpenAI behind adapter boundaries rather than inside the services themselves.
- The storage layer is intentionally split between workflow state and retrieval state.
- `agent_actions` is now the first durable shared-identity seam for GigiDC. It captures explicit Gigi-mediated relays and follow-up tasks that can be recalled later by the participants.
- DM tool execution now sits on top of that same shared-identity seam, so task and relay actions created from DM can still be audited, recalled, and permission-checked consistently.
- Infrastructure is not incidental. Terraform, bootstrap scripts, release scripts, and CI/CD form a distinct operations surface around the bot.

## Keep This Updated When

- command surfaces change
- retrieval flow changes
- storage or schema boundaries change
- deployment topology changes
