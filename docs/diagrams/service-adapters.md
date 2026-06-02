---
title: Service and Adapter Boundaries
description: Ports-and-adapters view of the Go foundation seams.
---

# Service And Adapter Boundaries

This diagram captures the current foundation seams. Discord liveness routing is live; capability, identity, audit, plugin, job, and LLM packages provide contracts or foundations for later privileged behavior.

```mermaid
flowchart LR
  App["internal/app"]
  Web["internal/web"]
  Config["internal/config"]
  Storage["internal/storage"]
  Discord["internal/discord<br/>gateway + liveness routing"]
  Capability["internal/capability<br/>role/user grants"]
  Identity["internal/identity<br/>sync resolver contract"]
  Audit["internal/audit<br/>event + store seam"]
  Plugins["internal/plugins<br/>manifest + registry contract"]
  Jobs["internal/jobs<br/>queue contract"]
  LLM["internal/llm<br/>text client contract"]
  DB["PostgreSQL + pgvector"]
  Providers["Future Providers<br/>Discord API + OpenAI-compatible APIs"]

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
- `internal/storage` currently checks DB reachability without taking a SQL dependency yet.
- `internal/capability` evaluates user and role grants by Discord IDs, with explicit admin override.
- `internal/identity` defines fail-closed identity resolution for future privileged actions.
- `internal/audit` validates audit events and provides a durable audit-log store seam.
- `internal/plugins` defines plugin manifest shape: capabilities, triggers, surfaces, permissions, config schema, and attribution.
- `internal/jobs` defines durable work records before workers exist.
- `internal/discord` has live `/ping`, DM `ping`, and mention `ping`; rich chat and privileged actions are still future work.
- `internal/llm` is a narrow contract only; no provider API call happens in this slice.
