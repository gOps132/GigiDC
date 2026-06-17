# V0 LLM Provider HTTP Client Implementation Plan

**Goal:** Add concrete non-streaming text-generation clients for configured OpenAI, Anthropic, and Gemini credentials so the later cognitive fallback can call real providers through resolved model credentials.

**Architecture:** `internal/llm` owns provider HTTP calls. `internal/llm/provider` still owns credential resolution and usage storage. This slice does not wire Discord chat yet; it creates the safe provider-call adapter and tests.

## Steps

- [x] Check current official provider request/response shapes for OpenAI Responses, Anthropic Messages, and Gemini GenerateContent.
- [x] Add resolved-model text request and HTTP provider client types.
- [x] Implement OpenAI Responses API request/response parsing.
- [x] Implement Anthropic Messages API request/response parsing.
- [x] Implement Gemini GenerateContent request/response parsing.
- [x] Add sanitized provider error handling.
- [x] Add HTTP unit tests for request shape, response parsing, token usage, and secret/body leakage guards.
- [x] Run `go test ./internal/llm`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
