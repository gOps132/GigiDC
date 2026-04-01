---
title: Knowledge Workflow
description: Lightweight domain-based insight capture for reusable project knowledge.
---

# Knowledge Workflow

This repo can support a lightweight knowledge-capture loop, but it should complement the existing `AGENTS.md` memory rules instead of competing with them.

## Recommendation

Use this workflow for meaningful tasks that produced reusable knowledge.

Do not require a write-up for trivial edits, routine formatting, or one-off local churn. That would create noise faster than it creates value.

## Directory Layout

Store reusable learnings under `knowledge/`:

```text
knowledge/
  INDEX.md
  _template/
    knowledge.md
    hypotheses.md
    rules.md
  <domain>/
    knowledge.md
    hypotheses.md
    rules.md
```

Recommended domain names are stable areas such as:

- `discord`
- `retrieval`
- `permissions`
- `deploy`
- `pricing`
- `workflows`

Use one domain per durable topic, not one folder per ticket.

## File Roles

### `knowledge.md`

Store durable facts, observed patterns, and concise notes from completed work.

Good examples:

- recurring failure patterns
- implementation constraints
- observed behavior from tests or production debugging
- workflow lessons that are likely to matter again

### `hypotheses.md`

Store ideas that look useful but still need more evidence.

Each hypothesis should include:

- the claim
- why it seems plausible
- what evidence would confirm or reject it
- the current confirmation count

### `rules.md`

Store confirmed defaults the team should apply automatically.

A rule should be short, operational, and safe to reuse without re-deriving the logic every time.

## Promotion And Demotion

- promote a hypothesis to `rules.md` after 5 independent confirmations
- if a rule is contradicted by new evidence, move it back to `hypotheses.md`
- update the confirmation count when new tasks strengthen or weaken confidence

The promotion threshold should be treated as a default, not a loophole. If a pattern is security-critical or very costly to repeat, also consider recording it in `AGENTS.md`.

## Relationship To `AGENTS.md`

Keep the two systems distinct:

- `knowledge/` is the working library for domain-specific facts, patterns, hypotheses, and rules
- `AGENTS.md` is the project-wide operating manual for durable, high-impact guidance

Promote an item into `AGENTS.md` when it is:

- recurring
- costly
- security-relevant
- easy to repeat by accident
- important enough that every future agent should see it immediately

Do not duplicate long entries across both places unless there is a strong reason.

## End-Of-Task Loop

At the end of each meaningful task:

1. identify the domain that learned the most
2. add confirmed observations to that domain's `knowledge.md`
3. add uncertain but promising patterns to `hypotheses.md`
4. promote or demote items if the evidence threshold changed
5. update `knowledge/INDEX.md`
6. if the lesson is project-wide and durable, add it to `AGENTS.md`

## Guardrails

- never store secrets, tokens, private message content, or sensitive identifiers in `knowledge/`
- prefer sanitized summaries over raw logs
- keep entries concise and decision-useful
- avoid turning `knowledge/` into a ticket archive or daily journal

## Current Adaptation For GigiDC

For this repo, the recommended adaptation is:

- use `knowledge/` for cross-task reusable learnings by domain
- keep `AGENTS.md` as the strongest project memory layer
- default to capturing insights after meaningful tasks, not literally every tiny action
- use the `workflows` domain first for process lessons until other domains accumulate enough signal
