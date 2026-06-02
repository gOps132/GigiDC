---
title: System Overview
description: High-level view of the Go foundation runtime.
---

# System Overview

This overview reflects the current Go foundation. Discord, LLM, retrieval, and plugin execution are typed seams only; they do not run yet.

```mermaid
flowchart LR
  Env["Environment Config"]
  App["cmd/gigi + internal/app"]
  Web["internal/web<br/>/healthz + /readyz"]
  Ready["internal/storage<br/>TCP DB readiness"]
  DB["Local PostgreSQL + pgvector"]
  Seams["Typed Future Seams<br/>discord plugins jobs storage llm"]
  Compose["Docker Compose"]
  CI["GitHub Actions<br/>Go + Compose smoke"]
  Deploy["Coolify / Docker Deploy"]

  Env --> App
  App --> Web
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
- Future Discord, plugin, job, storage, and LLM behavior is represented as package contracts, not active behavior.
- Docker Compose is the local and production deployment shape.

## Keep This Updated When

- command surfaces become live
- plugin execution becomes live
- job workers become live
- storage schema boundaries change
- deployment topology changes
