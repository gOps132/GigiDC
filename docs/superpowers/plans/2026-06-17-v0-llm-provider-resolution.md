# V0 LLM Provider Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve an active guild model profile into provider, model, params, billing owner, and plaintext credential for internal LLM calls without owner fallback.

**Architecture:** `internal/llm/provider` owns purpose-based resolution. Callers provide owner scope and purpose; service loads the active profile, loads the exact same-owner credential, opens the sealed secret, and returns a `ResolvedModel` for downstream provider clients.

**Tech Stack:** Go, SQL store methods, AES-GCM sealer, table-driven tests.

---

## Task 1: Store

- [ ] Add active credential lookup by owner, credential ID, and provider ID.
- [ ] Include sealed bytes only in this resolver-only lookup.
- [ ] Require active and non-revoked credentials.
- [ ] Preserve no-fallback owner matching.

## Task 2: Service

- [ ] Add `ResolveActiveModel`.
- [ ] Validate owner, purpose, and actor.
- [ ] Load active profile for the exact owner and purpose.
- [ ] Load matching credential for that same owner.
- [ ] Open secret with canonical provider/label AAD.
- [ ] Return provider, model, params, credential metadata, API key, and billing owner.

## Task 3: Verification

- [ ] Run `go test ./internal/llm/provider`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
