# V0 LLM Policy Placeholder Implementation Plan

**Goal:** Add the policy data foundation that keeps personal BYOK off by default without enabling personal-key behavior in V0.

**Architecture:** `internal/llm/provider` owns guild policy storage. The migration defaults `personal_keys_mode` to `off` and validates future modes, but no `/llm key` behavior is enabled in V0.

## Steps

- [x] Add `llm_guild_policies` migration with `personal_keys_mode default 'off'`.
- [x] Add storage schema test for policy defaults and allowed modes.
- [x] Add provider policy model and SQL store.
- [x] Add policy store tests for default-off lookup and upsert validation.
- [x] Run `go test ./internal/llm/provider ./internal/storage`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
