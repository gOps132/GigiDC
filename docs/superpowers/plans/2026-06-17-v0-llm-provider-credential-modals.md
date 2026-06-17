# V0 LLM Provider Credential Modals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let authorized guild admins add or rotate provider credentials through Discord modals without exposing raw keys in slash command options, audit metadata, responses, or modal custom IDs.

**Architecture:** `internal/discord` extends command routing to support modal responses and modal-submit interactions. `/llm provider add|rotate` stores short-lived opaque pending modal state server-side and only places an unguessable nonce in `custom_id`.

**Tech Stack:** Go, discordgo modal components, in-memory pending modal store, provider service, capability re-checks.

---

## Task 1: Router Modal Support

- [ ] Extend `CommandResponse` with modal response data.
- [ ] Send `InteractionResponseModal` when a command returns a modal.
- [ ] Route `InteractionModalSubmit` by custom ID prefix.
- [ ] Extract modal text input values with safe component walking.
- [ ] Re-run capability authorization on modal submit.

## Task 2: Credential Modal Flow

- [ ] Add `AddCredential` and `RotateCredential` to the Discord provider manager interface.
- [ ] Enable `/llm provider add` and `rotate` only when credential entry is configured.
- [ ] Store pending modal data server-side with guild, actor, action, provider, label, expiry.
- [ ] Use opaque nonce-only modal custom IDs.
- [ ] Consume pending modal state exactly once.
- [ ] Reject actor/guild mismatch and expired/replayed submits.

## Task 3: Safety

- [ ] Reject secret-looking credential labels.
- [ ] Never include raw key in slash options, modal custom ID, response, or audit metadata.
- [ ] Audit add/rotate with provider/action only.
- [ ] Re-check `llm.provider.write` on modal submit.

## Task 4: Verification

- [ ] Run `go test ./internal/discord`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
