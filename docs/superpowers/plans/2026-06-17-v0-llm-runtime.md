# V0 LLM Runtime Implementation Plan

**Goal:** Add a small orchestration service that resolves the active model, calls the provider text client, and records usage without storing prompt or completion text.

**Architecture:** `internal/llm` owns this runtime wrapper. `internal/llm/provider` still owns credential resolution and usage persistence. Discord/cognitive handlers will consume this runtime in later slices.

## Steps

- [x] Add resolver and runtime service interfaces/types.
- [x] Validate actor, owner, purpose, and input at runtime boundary.
- [x] Resolve active model before provider call.
- [x] Call `ResolvedTextClient`.
- [x] Record successful usage with provider/model/billing owner/tokens.
- [x] Record failed provider calls with safe error class and no prompt/completion text.
- [x] Add unit tests for success, failure recording, validation, and usage write failure.
- [x] Run `go test ./internal/llm`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
