---
name: project-visualization
display_name: Project Visualization
description: Keep repo understanding current with Understand-Anything analysis, refreshed architecture docs, and durable diagrams.
category: Documentation & Analysis
platforms: [codex]
version: 1.0.0
author: Gigi
tags: [architecture, diagrams, understand-anything]
---

# Project Visualization

## When to Use This Skill

Use this skill when you need to:

- understand the architecture of this repo quickly
- assess cross-cutting change impact before implementation or review
- keep visual diagrams current as the project evolves
- refresh onboarding material after architectural changes

## Required Workflow

1. If `Understand-Anything` is installed and the change is broad enough to justify it, start with its skills:
   - `understand-onboard` for repo onboarding
   - `understand-dashboard` for the visual graph
   - `understand-explain` for specific modules
   - `understand-diff` for cross-cutting changes
2. Use local code inspection to verify the touched paths instead of trusting generated summaries blindly.
3. If the architecture or system flow changed, update:
   - `docs/architecture-v1.md`
   - `docs/diagrams/`
   - `docs/project-visualization-workflow.md` when the workflow itself changes
4. If a new external tool or provider was used, update `docs/credits.md`.
5. Review all generated diagrams and summaries for secrets, private data, and misleading claims before commit or sharing.

For small local changes, skip `Understand-Anything` and rely on direct code inspection plus the maintained docs instead.

## Fallback

If `Understand-Anything` is not installed or unavailable:

- inspect the repo directly
- use the text architecture docs as the source of truth
- still refresh diagrams and docs when the architecture changes

## Notes

- Treat generated project graphs as aids, not source of truth.
- Prefer durable diagrams that future contributors can keep current without special tooling.
