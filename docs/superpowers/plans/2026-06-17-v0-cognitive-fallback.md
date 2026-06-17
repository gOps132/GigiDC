# V0 Cognitive Fallback Implementation Plan

**Goal:** Add a minimal LLM-backed fallback after deterministic external app matching fails, with guild mentions using the guild `chat` model profile.

**Architecture:** `internal/assistant` owns surface-independent assistant behavior. `internal/discord` adapts Discord messages into assistant requests. LLM output is plain chat text only in this slice; semantic plugin routing and action planning remain later.

## Steps

- [x] Add assistant message/responder types.
- [x] Implement guild-mention chat via `llm.Runtime.GenerateText`.
- [x] Keep DMs safe until user/tenant credential policy exists.
- [x] Add Discord message-handler adapter.
- [x] Wire assistant fallback into app after `ExternalAppDryRunHandler`.
- [x] Add assistant and Discord adapter tests.
- [x] Run `go test ./internal/assistant ./internal/discord ./internal/app`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
