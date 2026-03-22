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
  Fix or required workflow: Prefer the GitHub Actions release-based deploy pipeline instead of `git pull` on the server, and keep `/opt/gigi-discord-bot` owned by the service user when manual intervention is required.
  Verification step: Run a workflow deploy, confirm the release installs without git access on the EC2, and verify `sudo systemctl status gigi-discord-bot --no-pager` stays healthy afterward.
