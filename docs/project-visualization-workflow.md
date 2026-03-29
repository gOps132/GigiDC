# Project Visualization Workflow

This repo uses `Understand-Anything` as the primary codebase-visualization tool and keeps human-maintained diagrams alongside it.

## Goals

- keep a live code understanding workflow available during development
- keep architecture and feature diagrams current enough to support onboarding and refactors
- make diagram updates part of normal delivery instead of a one-off cleanup task
- preserve attribution and sensitive-data review for generated diagrams and graph outputs

## Tooling

- `Understand-Anything`
  - use for codebase graph generation, dashboard views, onboarding, architecture explanation, and diff-impact analysis
- repo-local `project-visualization` skill
  - use for the repo's standard analysis and refresh workflow
- `docs/architecture-v1.md`
  - keep the text-based architecture view current
- `docs/diagrams/`
  - store durable visual diagrams that help humans understand the system

## Setup

Install `Understand-Anything` locally:

```bash
bash scripts/setup-understand-anything.sh
```

Then restart Codex and verify the skills are visible:

```bash
ls -la ~/.agents/skills | grep understand
```

## Standard Workflow

Use this workflow for onboarding, architecture work, large feature work, refactors, and cross-cutting bug fixes.

### 1. Analyze the current codebase

- Start with the graph, not a broad docs-reading pass.
- Use `understand` or the current `.understand-anything/knowledge-graph.json` baseline first to identify actual entrypoints, layers, and dependency seams.
- Then read the smallest relevant set of docs for product intent, operational exceptions, or planned future changes.
- Use `understand-onboard` when entering the repo or when the architecture has changed materially.
- Use `understand-dashboard` when you need the visual graph or dependency map.
- Use `understand-explain` on specific modules before touching them.

Typical prompts:

- `Analyze this codebase and build a knowledge graph.`
- `Help me understand this project's architecture.`
- `Explain the assignment flow and the retrieval pipeline.`
- `Use the graph first, then tell me which docs are worth reading.`

### 2. Update the graph before and after significant changes

- Use `understand` or `understand-dashboard` to refresh the graph after material code changes.
- Use `understand-diff` before merge or review when a change affects multiple modules or boundaries.

Use diff analysis for:

- command and service boundary changes
- schema and retrieval changes
- deployment or infrastructure workflow changes
- anything that changes the mental model of the system

### 3. Refresh human-facing architecture diagrams

After any change that alters the architecture, system flow, permissions model, storage layout, or deployment topology:

- update [architecture-v1.md](/Users/giancedrick/dev/projects/gigi/docs/architecture-v1.md)
- update or add a diagram in `docs/diagrams/`
- keep the diagram title and surrounding doc language aligned with actual code paths

Suggested diagram filenames:

- `docs/diagrams/system-overview.excalidraw`
- `docs/diagrams/assignment-publish-flow.excalidraw`
- `docs/diagrams/dm-retrieval-flow.excalidraw`
- `docs/diagrams/deploy-release-flow.excalidraw`

### 4. Update documentation and attribution

When the workflow introduces or depends on a new external tool, skill, library, or generated artifact source:

- update [credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md)
- update setup or workflow docs when install steps or usage expectations change

### 5. Review for secrets and private data

Before committing diagrams, exports, screenshots, or generated summaries:

- remove secrets, tokens, internal-only URLs, or user data
- avoid embedding raw DM content or sensitive operational data into diagrams
- treat generated graph labels and screenshots as possible leak surfaces

## Commit Checklist

Before finalizing architecture-impacting work:

- run the relevant code validation steps
- refresh the `Understand-Anything` view for the touched area
- update `docs/architecture-v1.md` if the mental model changed
- update `docs/diagrams/` if a visual explanation would help future work
- update `docs/credits.md` if a new external source was introduced

## When To Skip Diagram Refresh

You usually do not need to refresh diagrams for:

- copy-only documentation edits
- comment-only code changes
- trivial typo fixes
- strictly local refactors that do not change structure, responsibilities, or flow
