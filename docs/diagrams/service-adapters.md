---
title: Service and Adapter Boundaries
description: Ports-and-adapters view of the Go foundation seams.
---

# Service And Adapter Boundaries

This diagram captures the current foundation seams. Discord liveness routing, permission grants, guild-scoped LLM controls, plugin catalog controls, external app dry-run matching, semantic dry-run routing, guild mention chat fallback, and consented opt-in public dispatch are live when configured; capability, identity, audit, external app manifest, job, retrieval, and memory packages provide contracts or foundations for later privileged behavior.

```mermaid
flowchart LR
  App["internal/app"]
  Web["internal/web"]
  Config["internal/config"]
  Storage["internal/storage"]
  Discord["internal/discord<br/>gateway + liveness + permissions + llm + plugins + dry-run + public dispatch"]
  Capability["internal/capability<br/>role/user grants"]
  Identity["internal/identity<br/>sync resolver contract"]
  Audit["internal/audit<br/>event + store seam"]
  Plugins["internal/plugins<br/>external app manifest contract"]
  Jobs["internal/jobs<br/>queue contract"]
  LLM["internal/llm<br/>provider-backed text runtime"]
  DB["PostgreSQL + pgvector"]
  Providers["External Providers<br/>Discord API + OpenAI + Anthropic + Gemini"]

  App --> Web
  App --> Config
  Web --> Storage
  Storage --> DB
  App --> Discord
  Discord --> Identity
  Discord --> Capability
  Discord --> Audit
  App --> Plugins
  App --> Jobs
  App --> LLM
  Discord --> Providers
  LLM --> Providers
```

## Reading Guide

- `internal/app` owns process lifecycle and graceful shutdown.
- `internal/web` owns HTTP health/readiness only.
- `internal/storage` checks DB reachability and runs idempotent migration files at startup.
- `internal/capability` evaluates and manages user/role grants by Discord IDs, with explicit admin override.
- `internal/identity` defines fail-closed identity resolution for future privileged actions.
- `internal/audit` validates audit events and provides a durable audit-log store seam; `internal/agent` also persists sanitized run and step records for agent executions.
- `internal/plugins` defines external app manifest shape: capabilities, triggers, surfaces, permissions, config schema, and attribution.
- `internal/jobs` defines durable work records before workers exist.
- `internal/discord` has live `/ping`, DM `ping`, mention `ping`, `/permissions`, `/llm`, `/plugins`, external app dry-run planning, semantic dry-run routing, guild mention chat fallback, and consented public `send_message` prefix dispatch; rich DM chat and restricted dispatch are still future work.
- `internal/llm` resolves guild model profiles, calls configured providers, and records usage without storing raw prompts or completions.
