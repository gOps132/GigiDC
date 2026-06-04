---
title: System Overview
description: High-level view of the Go foundation runtime.
---

# System Overview

This overview reflects the current Go foundation. Health/readiness, Discord liveness routing, and permission grants run today; LLM, retrieval, and external app command execution remain foundations for later privileged behavior.

```mermaid
flowchart LR
  Env["Environment Config"]
  App["cmd/gigi + internal/app"]
  Web["internal/web<br/>/healthz + /readyz"]
  Ready["internal/storage<br/>DB readiness + migrations"]
  DB["Local PostgreSQL + pgvector"]
  Discord["Discord Gateway<br/>/ping + DM/mention ping + /permissions"]
  Security["Capability + Identity + Audit<br/>permission gate"]
  Seams["Future Seams<br/>external apps jobs retrieval llm"]
  Compose["Docker Compose"]
  CI["GitHub Actions<br/>Go + Compose smoke"]
  Deploy["Coolify / Docker Deploy"]

  Env --> App
  App --> Web
  App --> Discord
  Discord --> Security
  Web --> Ready
  Ready --> DB
  App --> Seams
  Compose --> App
  Compose --> DB
  CI --> Compose
  Deploy --> Compose
```

## Reading Guide

- The runtime starts from `cmd/gigi`, loads config, and serves HTTP.
- `/healthz` reports process/build health.
- `/readyz` fails closed unless required config exists and PostgreSQL is reachable.
- Discord liveness behavior is active when Discord is enabled.
- Capability, identity, and audit gate `/permissions`; external app, job, retrieval, and LLM packages are foundations for later privileged behavior.
- Docker Compose is the local and production deployment shape.

## Keep This Updated When

- command surfaces become live
- external app command execution becomes live
- job workers become live
- storage schema boundaries change
- deployment topology changes
