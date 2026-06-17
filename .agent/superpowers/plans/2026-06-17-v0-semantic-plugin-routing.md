# V0 Semantic Plugin Routing Implementation Plan

**Goal:** Add LLM-assisted plugin routing as dry-run only after deterministic prefix matching fails.

**Architecture:** LLM output is only a proposal. Gigi validates the proposed plugin ID and trigger against enabled manifests, rebuilds the command from manifest data, applies capability checks, records safe audit metadata, and never dispatches semantic plans in V0.

## Steps

- [x] Add manifest-grounded `PlanCommandFromTrigger`.
- [x] Add semantic plugin planner that requests JSON hints from the routing profile.
- [x] Reject invented plugin IDs or triggers.
- [x] Integrate semantic routing after deterministic matching misses.
- [x] Keep semantic routing dry-run only, even for public send-message manifests.
- [x] Wire semantic planner in app.
- [x] Run `go test ./internal/plugins ./internal/assistant ./internal/discord ./internal/app`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
