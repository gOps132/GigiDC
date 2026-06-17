# V0 Conversation Metadata Implementation Plan

**Goal:** Add conversation persistence foundation without storing raw user prompts or model completions.

**Architecture:** `internal/assistant` records metadata-only user/assistant turns after LLM replies. The migration stores request, surface, actor, provider/model, and character counts only. Raw text storage stays deferred until explicit memory/privacy policy exists.

## Steps

- [x] Add metadata-only conversation-turn migration.
- [x] Add storage schema test that forbids raw text/prompt/completion columns.
- [x] Add `SQLConversationStore` and validation.
- [x] Add conversation store tests.
- [x] Add runtime response metadata for request/provider/model linkage.
- [x] Record user and assistant turn metadata from assistant handler.
- [x] Wire conversation store in app.
- [x] Run `go test ./internal/llm ./internal/assistant ./internal/storage ./internal/app`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
