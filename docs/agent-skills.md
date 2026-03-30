---
title: Agent Skills
description: Repo-local skills and how they fit the GigiDC workflow.
---

# Project Workflow Skills

This repo keeps project-specific agent skills in `.codex/skills` so the workflow stays close to the codebase instead of depending on whatever happens to be installed globally.

## Installed Skills

- `discord-bot`
  - Use for Discord bot implementation work, especially slash commands, interaction handling, roles, and other Discord-specific patterns
- `project-visualization`
  - Use for codebase understanding, architecture refresh work, visual diagram maintenance, and cross-cutting change analysis
  - Pair it with `Understand-Anything` selectively for onboarding, large refactors, or cross-cutting architecture work after running `bash scripts/setup-understand-anything.sh`
- `supabase-postgres-best-practices`
  - Use for Supabase/Postgres schema design, migrations, SQL queries, indexes, performance work, and RLS reviews
- `terraform-style-guide`
  - Use for Terraform authoring and review
  - Preserve this repo's Terraform 1.5.x compatibility rule from `AGENTS.md`; do not raise `required_version` just because a skill example uses a newer release
- `security-best-practices`
  - Use for security reviews and secure-by-default changes involving secrets, Discord permissions, DM history, OpenAI requests, or Supabase access boundaries
- `github-workflow`
  - Use for `gh`-based PR workflows, branching strategy, merge flow, and general GitHub operations
- `gh-fix-ci`
  - Use when a pull request has failing GitHub Actions checks and the failure needs to be inspected, summarized, and fixed
- `gh-address-comments`
  - Use when addressing review threads or issue comments on an open GitHub pull request

## Why These Skills

These were selected for this repo's actual stack and workflow:

- TypeScript + Node 22 application code
- Discord bot integration
- ongoing architecture understanding and project visualization
- Supabase/Postgres data model and retrieval features
- Terraform-managed infrastructure
- GitHub Actions CI/CD and pull-request review flow
- Security-sensitive handling of Discord data, bot credentials, and service-role access

## Selection Rule

Prefer adding a small number of stack-relevant skills over installing broad collections. If a new skill is added later:

- make it repo-local in `.codex/skills` when it is specific to this project's workflow
- add attribution in `docs/credits.md`
- document when it should be used so it does not become dead weight

## External Visualization Tooling

This repo also standardizes on `Understand-Anything` for codebase graphing and dashboard-based exploration.

- Install it with `bash scripts/setup-understand-anything.sh`
- Follow [project-visualization-workflow.md](/Users/giancedrick/dev/projects/gigi/docs/project-visualization-workflow.md) for the standard `analyze -> update graph -> refresh diagrams -> update docs` loop
