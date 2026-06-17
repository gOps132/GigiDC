# V0 LLM Provider Test Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/llm provider test` verify stored credentials through provider-safe probe requests, update credential test metadata, and keep secret material out of responses and audit.

**Architecture:** `internal/llm/provider` owns secret retrieval, decryption, provider test probes, and status updates. `internal/discord` only invokes the service and formats safe ephemeral output.

**Tech Stack:** Go, SQL store methods, AES-GCM sealer, `net/http`, provider model-list endpoints, table-driven tests.

---

## Task 1: Store

- [ ] Add active credential lookup by owner + label that includes sealed bytes for testing only.
- [ ] Add test-result update for `last_test_status`, `last_tested_at`, and `last_error_code`.
- [ ] Keep normal credential listing metadata-only.

## Task 2: Service

- [ ] Add `TestCredential` service API.
- [ ] Open credential secrets with canonical provider/label AAD.
- [ ] Call pluggable credential tester.
- [ ] Update sanitized success/failure result.
- [ ] Never return plaintext or sealed bytes.

## Task 3: Provider Tester

- [ ] Probe OpenAI with `GET /v1/models` and bearer auth.
- [ ] Probe Anthropic with `GET /v1/models`, `x-api-key`, and `anthropic-version`.
- [ ] Probe Gemini with `GET /v1beta/models?key=...`.
- [ ] Map HTTP status and request failures to sanitized error codes.

## Task 4: Discord UX

- [ ] Wire `/llm provider test` to provider service.
- [ ] Audit test success/failure without label, credential ID, fingerprint, or raw errors.
- [ ] Keep responses ephemeral and secret-free.

## Task 5: Verification

- [ ] Run `go test ./internal/llm/provider ./internal/discord ./internal/app`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
