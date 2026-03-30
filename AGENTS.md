# Project Rules

## External Resource Attribution

Every external resource used by this project must be explicitly mentioned and properly credited in project documentation or the relevant deliverable.

This includes, but is not limited to:

- Third-party skills, templates, prompts, starter kits, and code snippets
- Libraries, SDKs, APIs, platforms, and hosted services
- Design references, documentation sources, datasets, and example content
- Generated assets or outputs derived from external tools or providers

Minimum attribution requirements:

- Name the external resource or provider
- Describe how it is used in the project
- Link to the original source when practical
- Preserve required license or usage notices when applicable

Do not merge or present externally sourced material as project-original work without attribution.

## Sensitive Data Review

Every line added, modified, generated, logged, exported, or shared by this project must be reviewed for sensitive data leakage.

This includes, but is not limited to:

- API keys, tokens, secrets, signing keys, and credentials
- Environment variables, connection strings, and internal service URLs
- Personal data, private notes, direct messages, browser session data, and uploaded files
- Internal repository content, proprietary prompts, hidden system instructions, and private artifacts

Minimum review requirements:

- Check code, configs, logs, prompts, generated output, and docs before commit, deploy, or external sharing
- Redact or exclude sensitive values instead of masking them inconsistently
- Prefer references to secrets via secure storage rather than inline values
- Treat browser context, model inputs, and model outputs as potential leak surfaces

Do not commit, print, upload, or expose sensitive data unless it is explicitly intended, authorized, and documented.

## Recommended Development Environment

Use these defaults unless the repo later defines a stricter local standard:

- Node.js 22 LTS as the primary runtime
- TypeScript for application code
- Bun is allowed as optional local tooling, but do not assume Bun is the production runtime
- Docker or an equivalent container runtime for isolated code execution and reproducible local services
- Terraform CLI for infrastructure work
- Supabase CLI for local schema and service workflows when Supabase is added
- AWS CLI for infrastructure verification and operational checks when AWS resources are involved
- GitHub CLI for repository, pull request, and workflow inspection when GitHub integration is involved

Environment configuration rules:

- Keep local secrets in uncommitted environment files or secure secret stores only
- Provide example env files with placeholder values, never real credentials
- Prefer reproducible script-based setup over manual workstation tweaks
- Document every required external dependency, version expectation, and setup step in project docs

## Repo-Local Agent Skills

Prefer the repo-local skills in `.codex/skills` when a task matches their domain instead of relying on generic workflow only.

- Use `discord-bot` for Discord bot workflows such as slash commands, embeds, roles, threads, and Discord integration patterns
- Use `project-visualization` for codebase understanding, architecture refresh work, visual diagrams, and cross-cutting change analysis
- Use `supabase-postgres-best-practices` for Supabase/Postgres schema, query, index, migration, and RLS work
- Use `terraform-style-guide` for Terraform authoring and review, but keep the repo's Terraform Core compatibility aligned with 1.5.x unless the repo intentionally upgrades
- Use `security-best-practices` for security reviews, secrets handling, auth boundaries, and DM/privacy-sensitive changes
- Use `mintlify` for the docs site, docs navigation, MDX page structure, and Mintlify-specific documentation UX work
- Use `github-workflow`, `gh-fix-ci`, and `gh-address-comments` for PR workflows, GitHub Actions failures, and review-comment resolution

See `docs/agent-skills.md` for the current project-specific skill map and selection rationale.

## Project Visualization Workflow

Use `Understand-Anything` as optional deep-analysis tooling for onboarding, large refactors, and cross-cutting architectural work when it adds more clarity than context cost.

For repo understanding and architecture extraction, prefer the smallest high-signal source first: relevant human-maintained docs for known areas, and the generated graph/dashboard for broad or unclear boundaries.

- use `Understand-Anything` when you need help identifying entrypoints, layers, data boundaries, and cross-cutting dependencies across multiple modules
- then read only the docs needed for intent, product context, operational exceptions, or future plans
- treat graph-derived structure as optional acceleration, not a replacement for intent-bearing docs

For changes that alter system boundaries, data flow, deployment flow, permissions, or storage behavior:

- refresh the relevant `Understand-Anything` view or diff analysis
- update `docs/architecture-v1.md` when the text architecture model changes
- update or add diagrams under `docs/diagrams/` when a visual explanation would reduce future onboarding or review cost
- update `docs/credits.md` when a new external visualization or generation tool is introduced

If `Understand-Anything` stops being worth the context cost for the current change:

- skip it and rely on direct code inspection plus the maintained architecture docs
- do not regenerate graph artifacts just for routine small changes

For architecture changes that increase memory, retrieval scope, shared identity, automation, or tool use:

- document the main downsides and operational implications, not just the intended design
- explicitly call out risks such as context rot, noisy retrieval, duplicated memory representations, privacy leakage, retention pressure, storage cost, embedding cost, and durability gaps
- record what the current design does not solve yet so future work does not mistake a partial seam for a complete system
- keep these tradeoffs in `docs/architecture-v1.md` and the relevant roadmap or diagram notes when the mental model changes

Use `docs/project-visualization-workflow.md` as the standing checklist.

## Architecture Documentation Rules

Keep one canonical home per documentation concern so project docs stay readable and do not drift.

- Keep [README.md](/Users/giancedrick/dev/projects/gigi/README.md) as a short project front door only: what GigiDC is, what it offers, high-signal visuals, and links to deeper docs.
- Keep [docs/docs.json](/Users/giancedrick/dev/projects/gigi/docs/docs.json) as the docs-site navigation and information architecture source of truth.
- Keep [docs/architecture-v1.md](/Users/giancedrick/dev/projects/gigi/docs/architecture-v1.md) as the detailed system model for runtime layers, data boundaries, permissions, memory, and tradeoffs.
- Keep operational instructions in focused docs such as [docs/setup.md](/Users/giancedrick/dev/projects/gigi/docs/setup.md), [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md), and [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md), not in the README.
- Keep end-user product behavior in focused user docs such as [docs/user-guide.mdx](/Users/giancedrick/dev/projects/gigi/docs/user-guide.mdx), [docs/discord-usage.mdx](/Users/giancedrick/dev/projects/gigi/docs/discord-usage.mdx), and [docs/permissions.mdx](/Users/giancedrick/dev/projects/gigi/docs/permissions.mdx), not buried only in architecture docs.
- Keep visuals in `docs/diagrams/` when they explain a durable system concept; use README visuals only for the shortest overview-level mental model.
- When architecture changes, update the doc that is the source of truth instead of copy-pasting the same explanation into multiple files.
- Make it explicit what is current behavior, what is a constraint, and what is still future work; do not blur shipped architecture with roadmap intent.
- Record tradeoffs, limits, and unresolved implications in the architecture docs whenever memory, permissions, automation, or sensitive-data behavior changes.
- When shipped user-facing behavior changes, update the user docs and Mintlify navigation in the same change or an immediate follow-up. Do not leave Discord usage, permissions, DM-only behavior, or slash-command behavior documented only in code.
- Review diagrams, screenshots, and prose for sensitive-data leakage before commit or sharing.

## Human-Agent Workflow

Use `docs/human-agent-workflow.md` as the standing guide for multi-terminal Codex collaboration, worktree usage, explicit scope ownership, and agent status reporting.

When multiple Codex terminals are active:

- keep each terminal on its own branch and Git worktree
- assign one explicit owned scope per terminal before editing
- avoid overlapping edits to the same files, shared contracts, or Supabase migrations
- use one coordinating terminal for integration, rebases, and final verification

When a human asks what scope a terminal is working on, answer concretely with the current objective, branch, worktree, owned files or subsystem, current task, blockers, and next validation step.

## Interaction Authority Model

Treat Discord interaction surface as presentation, not authority.

- DM, slash commands, buttons, and select menus are all valid control surfaces for the same underlying capability model
- never assume a guild/admin action must stay slash-only if the requester can be resolved to a guild member with the required capability
- never grant extra authority because a request came through DM; resolve guild membership, capability, and target resources explicitly
- keep target resolution conservative and fail closed for user, channel, role, and assignment references when a DM request is ambiguous
- direct user grants are allowed, but they should stay visible and auditable so they do not silently replace the Discord role model
- keep audit coverage aligned across surfaces so a DM-triggered admin action leaves the same trail as a slash-command-triggered admin action

## Testing Instructions

Every meaningful change should include a verification step proportionate to risk.

Testing rules:

- Run the smallest relevant test set first, then expand to broader validation as needed
- Add or update automated tests when behavior, permissions, parsing, integrations, or persistence logic changes
- For bug fixes, add a regression test whenever practical
- Do not rely on manual testing alone for authorization, persistence, or execution-safety behavior
- Treat generated output, prompts, browser automation, and sandbox execution as test surfaces

Expected verification flow:

- Run targeted unit or integration tests for the changed area
- Run lint, typecheck, and any project-standard validation before finalizing work
- Validate role restrictions, sensitive-data handling, and external integration boundaries when applicable
- Clearly report what was tested, what was not tested, and any remaining risk

## Pull Request Instructions

Pull requests should be small enough to review carefully and explicit about risk.

PR rules:

- Use a clear title that describes the user-visible or system-level change
- Summarize the intent, main implementation changes, and validation performed
- Call out external resources, references, or borrowed material with proper attribution
- Explicitly mention any security, secrets, privacy, or data-handling implications
- Include rollout notes for infrastructure, schema, permissions, or integration changes
- Highlight follow-up work separately from the merged scope

Before opening or updating a PR:

- Re-check all modified lines for sensitive data leakage
- Re-check all external materials for proper attribution and license handling
- Confirm tests relevant to the change were run or explain why they were not
- Confirm generated code or content was reviewed rather than merged blindly
- For architectural changes, explicitly note the main risks, tradeoffs, and unresolved implications in the PR or updated docs instead of describing only the happy path

## Self-Healing Project Memory

When a real project issue is encountered, solved, and likely to matter again, update this `AGENTS.md` file so the solution becomes part of the repo's standing guidance.

Use this rule to make the project self-healing:

- Add durable lessons, not one-off noise
- Record the problem pattern, root cause, fix, and prevention rule
- Prefer concise operational rules that future agents can apply immediately
- Update `AGENTS.md` in the same change or follow-up change that resolves the issue
- Do not overwrite prior guidance unless the new solution clearly supersedes it

When adding a remembered issue, use a short entry under a future `Known Issues and Fixes` section with:

- Issue or symptom
- Root cause
- Fix or required workflow
- Verification step

Only promote issues into memory when they are recurring, costly, security-relevant, or easy to repeat by accident.

## Known Issues and Fixes

- Issue or symptom: Discord slash command builders with subcommands fail strict TypeScript checks when command interfaces assume only `SlashCommandBuilder`.
  Root cause: `discord.js` subcommand builders use a narrower builder type, so a command registry typed too strictly rejects valid command definitions.
  Fix or required workflow: Type command metadata to accept both `SlashCommandBuilder` and `SlashCommandSubcommandsOnlyBuilder`, or another common command-builder interface that supports `toJSON()`.
  Verification step: Run `npm run typecheck` and confirm the command registry accepts both simple and subcommand-based commands.

- Issue or symptom: `interaction.guild` remains nullable in command handlers even after guild-only checks.
  Root cause: `discord.js` interaction helpers do not always narrow TypeScript nullability enough for strict mode.
  Fix or required workflow: Add an explicit `const guild = interaction.guild; if (!guild) { ... return; }` guard before using guild-dependent properties.
  Verification step: Run `npm run typecheck` and confirm guild-dependent command handlers compile without nullability errors.

- Issue or symptom: `terraform init` fails immediately with an unsupported Terraform Core version error on machines using Terraform 1.5.x.
  Root cause: The repo starter used a stricter `required_version` floor than needed for the current AWS EC2/security-group configuration.
  Fix or required workflow: Keep the Terraform version constraint compatible with 1.5.x unless the repo starts using features that truly require 1.6+.
  Verification step: Run `terraform init` in the `terraform/` directory and confirm the AWS provider installs successfully on Terraform 1.5.7 or newer.

- Issue or symptom: Discord admin users can hit foreign-key failures when creating assignments or other guild-scoped records.
  Root cause: Capability checks returned early for Discord administrators before the `guilds` table was upserted, so later inserts into tables like `assignments` referenced a missing guild row.
  Fix or required workflow: Upsert the guild row before any administrator short-circuit in permission checks so every guild-scoped command initializes local guild state first.
  Verification step: Run `npm run typecheck` and `npm run build`, then create an assignment as a Discord administrator and confirm no `assignments_guild_id_fkey` error occurs.

- Issue or symptom: Manual EC2 deploys fail with `dubious ownership`, `.git/FETCH_HEAD` permission errors, or mixed ownership under `/opt/gigi-discord-bot`.
  Root cause: Pulling or cloning the server checkout with different users or `sudo git` causes git metadata and working tree files to drift between owners.
  Fix or required workflow: Prefer the release-based GitHub Actions CD pipeline for normal deploys, and if manual intervention is required keep `/opt/gigi-discord-bot` owned by the intended deploy user and avoid `sudo git`.
  Verification step: Run a CD deploy or `bash scripts/deploy-discord-bot.sh`, then confirm `sudo systemctl status gigi-discord-bot --no-pager` stays healthy afterward.

- Issue or symptom: Infrastructure drift or broken app changes reach `main` without being caught early.
  Root cause: Manual local checks are easy to skip, and the repo previously had no active CI baseline for TypeScript or Terraform validation.
  Fix or required workflow: Keep a lightweight GitHub Actions CI workflow that runs `npm run typecheck`, `npm run build`, `terraform fmt -check`, and `terraform validate` on pushes and pull requests before adding more features.
  Verification step: Confirm the CI workflow passes on a branch or pull request and fails when a TypeScript compile error or Terraform formatting error is introduced.

- Issue or symptom: Gigi-mediated DM replies or relay deliveries can be missing from canonical memory if the bot relies only on Discord gateway echoes to observe its own outbound messages.
  Root cause: Bot-authored outbound messages are not guaranteed to be observed or processed in the same way as inbound user messages during runtime and restart boundaries.
  Fix or required workflow: Explicitly persist bot-authored outbound DMs through `MessageHistoryService` immediately after sending them, especially for relay deliveries and DM conversation replies.
  Verification step: Run `npm run test` and confirm relay and DM-conversation tests verify `storeBotAuthoredMessage` is called for successful bot-authored DM outputs.

- Issue or symptom: Supabase schema changes drift between local notes, SQL editor steps, and checked-in migration history.
  Root cause: The repo originally stored migration SQL files without a CLI-managed workflow, making it easy to apply changes manually without preserving a reproducible path.
  Fix or required workflow: Keep `supabase/config.toml` checked in, use the existing `supabase/migrations/` files as the baseline history, and create all future schema changes with `supabase migration new` instead of ad hoc SQL editor edits.
  Verification step: Run `supabase --version`, confirm `supabase/config.toml` exists, and use `supabase db push` or `supabase db reset` against the intended environment.

- Issue or symptom: A fresh Supabase CLI push would drop active control-plane tables from `004_cleanup_legacy_clawbot_tables.sql`.
  Root cause: The repo kept an older cleanup migration from an abandoned direction even after the bot architecture continued depending on `channel_ingestion_policies`.
  Fix or required workflow: Keep `004_cleanup_legacy_clawbot_tables.sql` as a checked-in no-op placeholder so migration numbering stays contiguous without deleting active schema.
  Verification step: Run `supabase db push --dry-run` and confirm `004_cleanup_legacy_clawbot_tables.sql` no longer contains destructive DDL.

- Issue or symptom: Natural-language DM tool requests can target the wrong person or wrong task if name resolution is too permissive.
  Root cause: DM tool execution works from freeform text, so fuzzy nickname matching or aggressive fallback can convert ambiguity into the wrong relay or task mutation.
  Fix or required workflow: Keep DM tool resolution conservative. Prefer self references, explicit mentions, task IDs, or exact Discord names, and fail closed when a user or task reference is ambiguous.
  Verification step: Run `npm run test` and confirm the DM tool service and DM conversation tests still pass, especially the permission and task-resolution paths.

- Issue or symptom: Plain DMs can appear dead when history storage, embeddings, or semantic retrieval fail.
  Root cause: The DM path previously depended on `storeDiscordMessage` succeeding before conversation handling, and retrieval attempted semantic search before it could fall back to a simple direct answer.
  Fix or required workflow: Let DM conversation handling continue even if history persistence fails, degrade retrieval gracefully when semantic search is unavailable, and send a best-effort fallback error reply when DM handling still throws.
  Verification step: Run `npm run test` and confirm the Discord DM handler and retrieval fallback tests pass, especially the storage-failure and semantic-search-failure cases.

- Issue or symptom: Mentioning Gigi in a guild channel can produce no response even though DM chat works.
  Root cause: The runtime originally only routed `MessageCreate` into conversation handling for DMs, while guild messages were treated as ingestion-only unless they were slash interactions.
  Fix or required workflow: Keep an explicit guild-mention conversation path in the message handler that uses the current channel as retrieval scope, continues even when ingestion is disabled for that channel, and refuses public execution of private or administrative actions.
  Verification step: Run the Discord client and conversation tests, then mention Gigi in a server channel and confirm it replies from channel context instead of silently doing nothing.

- Issue or symptom: Gigi can hallucinate unsupported tools or broader runtime capabilities in DM if capability questions are left entirely to the language model.
  Root cause: A generic assistant prompt invites the model to answer from prior expectations instead of the actual bot runtime surface.
  Fix or required workflow: Ground capability and unsupported-tool questions with deterministic DM-intent routing, and keep the retrieval prompt explicit about the real bot surface: DM chat, retrieval, bounded user memory, tasks, and permission-gated relays only.
  Verification step: Run the DM conversation and retrieval tests and confirm `what tools can you call?`, unsupported code-execution questions, and ingestion-status questions stay grounded to the actual runtime.

- Issue or symptom: Gigi can appear to ask for relay confirmation in DM but fail to send anything because the confirmation exists only in model language, not in persisted action state.
  Root cause: Cross-user relay requests were previously allowed to roleplay an approval step without any canonical pending action, owner check, expiry, or execution handoff.
  Fix or required workflow: Route cross-user relay requests through persisted `agent_actions` rows in `awaiting_confirmation`, use button-based confirm/cancel as the primary path, and only let free-text confirm/cancel resolve a single unambiguous pending action owned by the requester.
  Verification step: Create a relay request in Discord, confirm it, and verify the action moves through confirmation, audit, and delivery state instead of relying on conversational promise text.

- Issue or symptom: Guild/admin capabilities can drift into slash-command-only behavior even though the same authenticated user should be able to invoke them from DM.
  Root cause: It is easy to treat Discord surface as the permission boundary instead of treating guild identity and `role_policies` capabilities as the actual authority model.
  Fix or required workflow: Keep guild/admin execution behind shared services that resolve the requester's primary-guild membership, check the same capabilities across surfaces, and fail closed when channel, role, or assignment targets are ambiguous in DM.
  Verification step: Run targeted tests for the shared guild-admin service, then verify the same user can perform an allowed ingestion or assignment action from both slash command and DM while a user without the capability is denied in both surfaces.

- Issue or symptom: Raw secret values can leak into ordinary DM history if users try to teach Gigi sensitive data through normal chat.
  Root cause: The default DM pipeline stores messages for retrieval, so any secret sent through ordinary DM chat becomes normal history unless it is intercepted before storage.
  Fix or required workflow: Keep sensitive-data writes out of ordinary Discord chat, use the encrypted sensitive-data store plus the local admin script for writes, and bypass normal DM history storage when a message looks like a raw secret-write attempt.
  Verification step: Run the sensitive-data and Discord DM handler tests, then confirm a raw secret-write attempt is refused and skipped by normal message-history storage.
