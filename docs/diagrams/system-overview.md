# System Overview

This overview has been refreshed from the first real `Understand-Anything` graph pass. It reflects the current repo shape rather than the earlier placeholder runtime sketch.

```mermaid
flowchart LR
  Discord["Discord"]
  Register["Command Registration<br/>src/discord/registerCommands.ts"]
  Gateway["Gateway Runtime<br/>src/discord/client.ts"]
  Commands["Slash Commands<br/>src/commands/*"]
  DMFlow["DM Scope + Retrieval<br/>DmConversationService + RetrievalService"]
  History["Message Storage + Background Indexing<br/>MessageHistoryService + MessageIndexingService"]
  Admin["Assignment + Ingestion Admin<br/>Role + Audit Services"]
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
  Gateway --> History
  Commands --> Admin
  Admin --> Ports
  DMFlow --> Ports
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
- Most behavior sits in services, especially the DM retrieval path and the admin command path for assignments and ingestion policy.
- The service layer now depends on explicit ports, with Supabase and OpenAI behind adapter boundaries rather than inside the services themselves.
- The storage layer is intentionally split between workflow state and retrieval state.
- Infrastructure is not incidental. Terraform, bootstrap scripts, release scripts, and CI/CD form a distinct operations surface around the bot.

## Keep This Updated When

- command surfaces change
- retrieval flow changes
- storage or schema boundaries change
- deployment topology changes
