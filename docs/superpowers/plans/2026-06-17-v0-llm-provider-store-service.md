# V0 LLM Provider Store And Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build SQL-backed provider credential/profile persistence and config support for encrypted LLM provider management.

**Architecture:** `internal/llm/provider` owns store/service primitives. Config exposes a base64 32-byte secret key for AES-GCM sealing but does not require it until provider-management UX is live.

**Tech Stack:** Go, PostgreSQL SQL strings, AES-GCM, table-driven tests.

---

## Task 1: SQL Store

- [ ] Write failing tests for credential validation, upsert, revoke, list metadata, model profile selection, active profile lookup, and rollback.
- [ ] Run `go test ./internal/llm/provider` and verify red.
- [ ] Implement `SQLStore` for `llm_credentials` and `llm_model_profiles`.
- [ ] Run `go test ./internal/llm/provider` and verify green.

## Task 2: Secret Key Config

- [ ] Write failing config tests for `GIGI_LLM_SECRET_KEY_BASE64`, `GIGI_LLM_SECRET_KEY_ID`, valid 32-byte decode, invalid base64, and wrong length.
- [ ] Run `go test ./internal/config` and verify red.
- [ ] Add config fields and decode helper.
- [ ] Update `.env.example`.
- [ ] Run `go test ./internal/config` and verify green.

## Task 3: Provider Service

- [ ] Write failing tests for service add/list/revoke/select operations using fake store and fake sealer.
- [ ] Run `go test ./internal/llm/provider` and verify red.
- [ ] Implement service methods that seal secrets, fingerprint raw secrets, call store, and never return plaintext.
- [ ] Run `go test ./internal/llm/provider` and verify green.

## Task 4: Verification

- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.

