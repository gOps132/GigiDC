---
title: Architecture
description: Current architecture model for the Go foundation rebuild.
---

# Architecture

## Current State

Gigi is in a foundation rebuild. The old Node/Supabase runtime is gone. The current runtime is a Go service that exposes:

- `GET /healthz`
- `GET /readyz`
- build metadata through `internal/buildinfo`
- config loading through `internal/config`
- database reachability through `internal/storage`

Discord login, plugin execution, retrieval, and LLM calls are intentionally not active yet.

## Target Shape

```text
Discord Gateway
  -> Go runtime
     -> interaction and DM router
     -> capability engine
     -> plugin skill registry
     -> durable jobs and outbox
     -> PostgreSQL + pgvector
     -> LLM adapters
```

## Package Boundaries

- `cmd/gigi`: binary entrypoint and process lifecycle.
- `internal/app`: application wiring.
- `internal/config`: environment parsing.
- `internal/web`: health and readiness HTTP handlers.
- `internal/storage`: database reachability and future migration/storage seams.
- `internal/plugins`: approved plugin manifest and registry contracts.
- `internal/jobs`: durable job contracts.
- `internal/discord`: Discord client contracts for later slices.
- `internal/llm`: LLM client contracts for later slices.

## Data Boundary

Local PostgreSQL is the new source of truth. The first migration creates foundation tables for runtime metadata, plugin installs, jobs, and outbox events. Supabase is not part of the live runtime and no backfill is planned.

## Plugin Direction

Gigi will discover approved plugins from manifests. A guild admin can enable an approved plugin, then Gigi can route prefix commands, slash commands, buttons, mentions, DMs, or natural-language requests to that plugin after permission and config checks.

## Known Limits

- No Discord gateway connection yet.
- No music or `!play` implementation yet.
- No LLM calls yet.
- No retrieval or memory behavior yet.
- Readiness checks database TCP reachability only in this slice.
