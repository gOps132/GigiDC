# V0 LLM Docs Sync Implementation Plan

**Goal:** Keep command and architecture docs aligned with the now-live V0 provider/model/usage surface.

**Architecture:** Documentation names shipped behavior only as live. Planned policy and personal BYOK commands remain clearly marked as planned/V1.

## Steps

- [x] Update `docs/bot-commands.mdx` with live `/llm provider`, `/llm model`, and `/llm usage guild` entries.
- [x] Keep `/llm policy set` and `/llm key ...` commands marked planned.
- [x] Update `docs/architecture.md` so `internal/llm/provider` is no longer described as planned.
- [x] Update `docs/llm-cognitive-plan.mdx` with completed provider foundation and remaining V0 work.
- [x] Run docs diff review.
- [x] Run `git diff --check`.
