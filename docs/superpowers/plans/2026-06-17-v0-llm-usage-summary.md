# V0 LLM Usage Summary Implementation Plan

**Goal:** Ship `/llm usage guild` so authorized guild operators can see aggregate LLM usage without exposing prompts, completions, secrets, or provider responses.

**Architecture:** `internal/llm/provider` owns usage-event aggregation from `llm_usage_events`. `internal/discord` owns slash command routing and safe ephemeral formatting. The app wires the existing SQL usage recorder into the LLM command config.

## Steps

- [x] Add guild usage summary aggregate to `SQLUsageRecorder`.
- [x] Add provider package tests for aggregate totals, empty usage, and guild validation.
- [x] Register `/llm usage guild`.
- [x] Gate usage view with existing `llm.provider.select` capability for V0 read access.
- [x] Add Discord handler tests for command surface, capability, configured reporter, zero usage, and disabled reporter.
- [x] Wire usage reporter in `internal/app`.
- [x] Run `go test ./internal/llm/provider ./internal/discord ./internal/app`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
