---
title: Service and Adapter Boundaries
description: Ports-and-adapters view of the Go foundation seams.
---

# Service And Adapter Boundaries

This diagram captures the current foundation seams. Most packages are contracts first, so later Discord, plugin, job, and LLM work can attach without rewriting the process shell.

```mermaid
flowchart LR
  App["internal/app"]
  Web["internal/web"]
  Config["internal/config"]
  Storage["internal/storage"]
  Discord["internal/discord<br/>future gateway contract"]
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
- `internal/plugins` defines plugin manifest shape: capabilities, triggers, surfaces, permissions, config schema, and attribution.
- `internal/jobs` defines durable work records before workers exist.
- `internal/discord` and `internal/llm` are narrow contracts only; no provider login or API call happens in this slice.
