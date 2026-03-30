---
name: mintlify
description: Use when working on the GigiDC docs site, including docs pages, docs.json navigation, Mintlify components, and hosted documentation UX.
---

# Mintlify For GigiDC

Use this skill when the task involves the Mintlify docs site for this repo.

Examples:

- adding or editing docs pages under `docs/`
- updating `docs/docs.json`
- improving the docs homepage
- reorganizing navigation
- adding user-facing product docs
- refining visuals, cards, or Mintlify component usage
- checking that README stays a short GitHub front door while the real docs live in Mintlify

## Project-Specific Rules

Read these first:

- `docs/docs.json`
- `docs/index.mdx`
- `AGENTS.md`
- `docs/agent-skills.md`

Then read only the pages directly relevant to the requested change.

## Source Of Truth

For this project:

- `README.md` is the short GitHub overview only
- `docs/docs.json` is the docs-site navigation source of truth
- `docs/index.mdx` is the docs landing page
- `docs/user-guide.mdx`, `docs/discord-usage.mdx`, and `docs/permissions.mdx` are the user-facing product docs
- `docs/architecture-v1.md` is the deep technical architecture doc

Do not move architecture-heavy detail back into the README.

## How To Work

1. Start by reading `docs/docs.json` to understand navigation.
2. Prefer updating an existing page before creating a new one.
3. When adding a new page, add it to `docs/docs.json` in the correct group.
4. Keep user-facing docs focused on shipped behavior, not roadmap intent.
5. When product behavior changes, update the user docs in the same change or immediate follow-up.
6. Keep links in Mintlify pages root-relative where appropriate.

## Mintlify Guidance

- Prefer built-in Mintlify patterns and components over custom MDX complexity.
- Keep the docs homepage presentable and concise.
- Use cards, short sections, and high-signal structure instead of long walls of text.
- Keep docs navigation understandable for both end users and maintainers.

Always prefer current Mintlify documentation and the Mintlify MCP server over stale assumptions.

Current hosted docs:

- `https://gigi-f9937525.mintlify.app/`

Current Mintlify MCP endpoint:

- `https://mintlify.com/docs/mcp`

## Validation

For docs-only changes, run lightweight checks first:

- `git diff --check`
- validate `docs/docs.json` parses

If local preview is needed:

- `npm run docs:dev`

## Attribution

This repo-local skill is aligned with Mintlify’s documentation platform and official documentation workflow. Keep Mintlify credited in `docs/credits.md`.
