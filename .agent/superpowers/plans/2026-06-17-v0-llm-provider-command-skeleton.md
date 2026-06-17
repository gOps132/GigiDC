# V0 LLM Provider Command Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register safe guild-scoped `/llm` commands that expose provider/profile state and wire them to the provider service without accepting raw API keys in slash command options.

**Architecture:** `internal/discord` owns command parsing and response formatting. `internal/app` wires `provider.Service` into Discord when the real gateway is enabled. Secret entry remains deferred until Discord modal/private-flow support exists.

**Tech Stack:** Go, discordgo command options, provider service interfaces, table-driven tests.

---

## Task 1: Command Shape

- [ ] Add `/llm provider` subcommands for `list`, `add`, `test`, `rotate`, and `delete`.
- [ ] Add `/llm model` subcommands for `show` and `set`.
- [ ] Use dynamic capability mapping:
  - `llm.provider.write` for list/add/rotate/delete.
  - `llm.provider.test` for test.
  - `llm.provider.select` for model show/set.
- [ ] Keep add/rotate as modal-required placeholders; do not accept raw provider secrets as normal slash command strings.

## Task 2: Handler Behavior

- [ ] Implement guild-only parsing.
- [ ] Implement provider list with metadata-only formatting.
- [ ] Implement provider delete with explicit confirmation.
- [ ] Implement model show via active profile lookup.
- [ ] Implement model set by resolving active credentials by label, then selecting the profile.
- [ ] Audit mutating provider/model actions without storing labels, secrets, params JSON, prompts, or completions.

## Task 3: App Wiring

- [ ] Build optional AES-GCM sealer from config for future credential writes.
- [ ] Instantiate SQL provider store/service.
- [ ] Register `/llm` commands beside permissions and plugin commands.

## Task 4: Verification

- [ ] Run `go test ./internal/discord ./internal/app`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
