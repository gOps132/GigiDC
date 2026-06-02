# Retired Node Discord Bot Plan

This file used to describe the removed Node/Supabase Discord bot plan.

The active implementation target is now:

- [Architecture](./architecture-v1)
- [Roadmap](./roadmap)
- [CI/CD](./ci-cd)

## Current Status

Gigi has been reset to a Go foundation. The old runtime, old schema, old command handlers, and old Supabase-backed storage have been removed.

The first live public interface is HTTP only:

- `GET /healthz`
- `GET /readyz`

Discord login, message handling, plugin execution, jobs, retrieval, and LLM calls are future slices.

## Future Discord Direction

Future Discord work should build on the Go seams already present:

- `internal/discord` for gateway and interaction contracts
- `internal/plugins` for admin-installed plugin manifests
- `internal/jobs` for durable background work
- `internal/storage` for local PostgreSQL persistence
- `internal/llm` for provider-isolated model calls

No legacy backfill or dual-write is planned.
