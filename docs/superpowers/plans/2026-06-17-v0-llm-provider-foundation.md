# V0 LLM Provider Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first safe foundation for Gigi's LLM provider registry, credential ownership model, encrypted secrets, and command authorization support.

**Architecture:** V0 uses multi-owner data structures (`guild`, `user`, `tenant`) with guild-only product behavior. `internal/llm/provider` owns provider metadata and secret handling; Discord remains an adapter; `/llm` command UX will be added after dynamic command authorization and provider services exist.

**Tech Stack:** Go, PostgreSQL, pgvector-ready migrations, Discord slash commands via `discordgo`, AES-GCM from the Go standard library.

---

## File Structure

- `db/migrations/000003_llm_provider_credentials.sql`: LLM credential, model profile, and usage-event tables.
- `internal/storage/schema_test.go`: string-level migration guards matching the repo's current schema tests.
- `internal/llm/provider/registry.go`: provider IDs, purposes, owner types, provider specs, and validation.
- `internal/llm/provider/secret.go`: `SecretSealer`, AES-GCM sealer, sealed envelope, and secret fingerprinting.
- `internal/discord/commands.go`: dynamic required-capability callback for subcommand trees.
- `internal/discord/commands_test.go`: command-router tests for dynamic capability behavior.

## Task 1: Schema Foundation

- [x] Write failing storage schema tests for the planned migration.
- [x] Run `go test ./internal/storage` and verify the new test fails because `000003_llm_provider_credentials.sql` is missing.
- [x] Add the idempotent migration with `llm_credentials`, `llm_model_profiles`, and `llm_usage_events`.
- [x] Run `go test ./internal/storage` and verify it passes.

## Task 2: Provider Registry And Secret Sealer

- [x] Write failing tests for provider validation, purpose validation, owner validation, flexible model IDs, AES-GCM sealing, AAD mismatch rejection, and non-secret fingerprints.
- [x] Run `go test ./internal/llm/provider` and verify the package fails before implementation.
- [x] Implement minimal provider registry and secret sealer code.
- [x] Run `go test ./internal/llm/provider` and verify it passes.

## Task 3: Dynamic Command Authorization

- [x] Write failing Discord command-router tests for dynamic required capabilities.
- [x] Run `go test ./internal/discord` and verify failure before implementation.
- [x] Add a `RequiredCapabilityFor` callback to `discord.Command` and use it in `HandleInteraction`.
- [x] Run `go test ./internal/discord` and verify it passes.

## Task 4: Integration Review

- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Check `git diff` for accidental edits outside the planned files.
- [x] Update docs only if implementation behavior diverges from the planning docs.
