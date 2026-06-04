---
title: Service and Adapter Boundaries
description: Ports-and-adapters view of the Go foundation seams.
---

# Service And Adapter Boundaries

This diagram captures the current foundation seams. Discord liveness routing, permission grants, plugin catalog controls, and external app dry-run matching are live; capability, identity, audit, external app manifest, job, and LLM packages provide contracts or foundations for later privileged behavior.

```mermaid
flowchart LR
  App["internal/app"]
  Web["internal/web"]
  Config["internal/config"]
  Storage["internal/storage"]
  Discord["internal/discord<br/>gateway + liveness + permissions + plugins + dry-run"]
  Capability["internal/capability<br/>role/user grants"]
  Identity["internal/identity<br/>sync resolver contract"]
  Audit["internal/audit<br/>event + store seam"]
  Plugins["internal/plugins<br/>external app manifest contract"]
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
- `internal/storage` checks DB reachability and runs idempotent migration files at startup.
- `internal/capability` evaluates and manages user/role grants by Discord IDs, with explicit admin override.
- `internal/identity` defines fail-closed identity resolution for future privileged actions.
- `internal/audit` validates audit events and provides a durable audit-log store seam.
- `internal/plugins` defines external app manifest shape: capabilities, triggers, surfaces, permissions, config schema, and attribution.
- `internal/jobs` defines durable work records before workers exist.
- `internal/discord` has live `/ping`, DM `ping`, mention `ping`, `/permissions`, `/plugins`, and external app dry-run planning; rich chat and command dispatch are still future work.
- `internal/llm` is a narrow contract only; no provider API call happens in this slice.
