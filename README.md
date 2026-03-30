# Gigi Discord Bot

DM-first Discord bot for agentic chat, scoped history retrieval, shared Gigi identity, durable task/action memory, and assignment notifications.

## What Is In This Repo

- A Node 22 + TypeScript Discord bot foundation
- DM-first agent experience backed by OpenAI
- Shared Gigi task/action memory for relays, follow-up questions, and tracked work
- Explicit relay confirmation flow plus bounded requester-centric user memory
- Bounded DM tool execution for task, relay, permission, ingestion, and assignment workflows
- Slash-command assignment notifier workflow, plus DM access to the same guild capabilities
- Direct user capability grants alongside role-based capability mappings
- Encrypted sensitive-data records with DM-only disclosure for the owning user
- Supabase/Postgres-backed control-plane state
- Raw message history + pgvector-ready retrieval foundation
- Project rules and planning docs

## Quick Start

### 1. Install dependencies

```bash
npm install
```

### 2. Create your local environment file

```bash
cp .env.example .env
```

Open `.env` and put your real API keys and IDs there:

- `DISCORD_TOKEN`
- `DISCORD_CLIENT_ID`
- `DISCORD_GUILD_ID`
- `PRIMARY_GUILD_ID`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `OPENAI_API_KEY`
- `SENSITIVE_DATA_ENCRYPTION_KEY` if you want encrypted sensitive-data retrieval

Do not commit `.env`.

### 3. Create the Discord application and bot

- Create an application in the Discord Developer Portal
- Create a bot user for that application
- Copy the bot token into `DISCORD_TOKEN`
- Copy the application client ID into `DISCORD_CLIENT_ID`
- Copy your server ID into `DISCORD_GUILD_ID` for guild-scoped slash command registration during development
- Invite the bot to your server with permissions to read, send, and use application commands

### 4. Configure Supabase and retrieval storage

- Create a Supabase project
- Copy the project URL into `SUPABASE_URL`
- Copy the server-side service role key into `SUPABASE_SERVICE_ROLE_KEY`
- Install the Supabase CLI
- Install Docker Desktop or another Docker-compatible runtime if you want the local Supabase stack
- This repo now includes `supabase/config.toml` and keeps database changes under `supabase/migrations/`
- Existing `001` to `005` files are the preserved baseline migrations for this project
- `004_cleanup_legacy_clawbot_tables.sql` is intentionally a no-op placeholder so the baseline history stays contiguous without dropping active tables
- For a linked remote project, apply them with `supabase db push`
- For local development, run `npm run supabase:start` and `npm run supabase:db:reset`
- Future schema changes should use `supabase migration new <name>` so new files follow the CLI-managed workflow

### 5. Configure OpenAI

- Set `OPENAI_API_KEY` to a valid OpenAI API key
- Optional: override `OPENAI_RESPONSE_MODEL` and `OPENAI_EMBEDDING_MODEL`
- The bot uses OpenAI for:
  - DM reasoning
  - semantic retrieval over stored chat history
  - no browser, OCR, or image understanding in V1

### 6. Start the bot

```bash
npm run dev
```

The bot can register slash commands at startup, starts a local health server on port `8080`, stores DM history immediately, stores guild history only for channels with ingestion explicitly enabled, persists participant-visible relay actions and tasks, keeps bounded requester-centric user memory snapshots, and responds to direct messages through deterministic DM routing plus bounded internal tool planning.

## EC2 Deployment

Deployment assets for the recommended two-EC2 layout are included in:

- [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)
- [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md)
- [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md)
- [deploy/systemd/gigi-discord-bot.service](/Users/giancedrick/dev/projects/gigi/deploy/systemd/gigi-discord-bot.service)
- [deploy/nginx/gigi-discord-bot.conf](/Users/giancedrick/dev/projects/gigi/deploy/nginx/gigi-discord-bot.conf)
- [scripts/bootstrap-ec2.sh](/Users/giancedrick/dev/projects/gigi/scripts/bootstrap-ec2.sh)
- [scripts/deploy-discord-bot.sh](/Users/giancedrick/dev/projects/gigi/scripts/deploy-discord-bot.sh)
- [scripts/install-release.sh](/Users/giancedrick/dev/projects/gigi/scripts/install-release.sh)
- [terraform/terraform.tfvars.example](/Users/giancedrick/dev/projects/gigi/terraform/terraform.tfvars.example)

## Developer Foundations

The repo now includes a CI workflow at [ci.yml](/Users/giancedrick/dev/projects/gigi/.github/workflows/ci.yml).

On every push to `main` and on every pull request, CI runs:

- `npm ci`
- `npm run typecheck`
- `npm run build`
- `terraform fmt -check`
- `terraform init -backend=false`
- `terraform validate`

On pushes to `main`, CD also:

- builds a release bundle
- uploads the release to the bot EC2 over SSH
- installs the release on the server
- restarts `gigi-discord-bot`

CD setup instructions and required GitHub secrets are in [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md).

## Available Commands and Interaction Modes

- `/ping`
- `/permission grant`
- `/permission revoke`
- `/permission list`
- `/ingestion enable`
- `/ingestion disable`
- `/ingestion status`
- `/relay dm`
- `/task create`
- `/task list`
- `/task complete`
- `/assignment create`
- `/assignment publish`
- `/assignment list`

DM the bot for:

- freeform questions
- semantic history search
- exact phrase count questions such as `How many times did I say "ship it"?`
- conversational follow-ups over your DM history
- follow-up questions about Gigi-mediated relay actions that involve you
- task-oriented questions like `what tasks do i still have?`
- explicit tool-style requests like `create a task for me to review launch notes tomorrow and show me my open tasks`
- explicit relay requests like `send Mina a DM saying the release moved to Friday`
- permission-gated ingestion admin requests like `show ingestion status for #shipping` or `enable ingestion for #shipping`
- permission-gated assignment admin requests like `create an assignment called Chapter 5 homework for @Students due 2026-04-02T09:00:00Z` or `publish assignment assignment-1`
- permission-management requests like `what permissions do i have?` or `grant permission_admin to @user`
- DM-only sensitive-data requests like `show my sensitive data` or `what is my github token`
- follow-up confirmations like `confirm!` or `cancel` when Gigi has exactly one pending relay waiting on you

The DM runtime does not provide:

- web browsing or web search
- code execution or a sandbox
- image generation
- arbitrary external tools

## Authorization Model

The bot checks Discord `Administrator` first, then capability mappings in `role_policies`.

Authority comes from guild identity and capability, not from whether the request arrived by slash command or DM.
DM is an alternate control surface, not a weaker permission model.

Capabilities used by the current command set:

- `agent_action_dispatch`
- `agent_action_receive`
- `assignment_admin`
- `ingestion_admin`
- `history_guild_wide`
- `permission_admin`

Example:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'agent_action_dispatch', 'your-shared-action-role-id'),
  ('your-discord-guild-id', 'agent_action_receive', 'your-gigi-dm-recipient-role-id'),
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'ingestion_admin', 'your-ingestion-admin-role-id'),
  ('your-discord-guild-id', 'history_guild_wide', 'your-history-enabled-role-id'),
  ('your-discord-guild-id', 'permission_admin', 'your-permission-admin-role-id');
```

You can also grant a capability directly to a specific user with `/permission grant`, which writes to `user_capability_grants`.

## Retrieval Model

V1 uses a reduced retrieval-first architecture:

- raw Discord messages are the source of truth
- DM history is always eligible for storage
- guild-channel ingestion is opt-in through `channel_ingestion_policies`
- direct one-off user grants are stored in `user_capability_grants`
- participant-visible Gigi actions and follow-up tasks are persisted in `agent_actions`
- cross-user DM relays move through an explicit `awaiting_confirmation -> in_progress -> completed/failed/cancelled` lifecycle
- requester-centric identity memory is stored in `user_profiles` and `user_memory_snapshots`
- encrypted sensitive records are stored in `sensitive_data_records`
- explicit tool-style DM requests can be planned into up to three internal tool calls before falling back to retrieval
- Gigi-mediated DM relays require sender dispatch permission and recipient receive permission
- DM-triggered ingestion and assignment admin actions use the same guild capability checks as their slash-command equivalents
- DM-triggered permission management uses the same `permission_admin` capability as the slash surface
- sensitive-data disclosure only happens in DM, is deterministic, and bypasses OpenAI completely
- ingestion policy changes and permission denials are written to `audit_logs`
- relay dispatch attempts and outcomes are written to `audit_logs`
- task creation and completion events are written to `audit_logs`
- DM-triggered ingestion and assignment admin actions are written to `audit_logs`
- direct permission grants and revocations are written to `audit_logs`
- DM scope-selection prompts are persisted briefly in Supabase so restarts do not invalidate active menus
- bot-authored DM replies and relay deliveries are stored explicitly in canonical message history instead of relying only on gateway echoes
- exact analytics use SQL/text search first
- semantic questions use OpenAI embeddings over stored messages
- history-aware DM answers can also draw from participant-visible relay actions and open tasks
- DM tool execution is intentionally bounded to task, relay, permission, ingestion, and assignment operations; there is still no browser worker, sandbox worker, or arbitrary external tool surface
- image attachments are stored as metadata only
- no OCR, autonomous memory promotion, or digest pipeline in V1
- raw sensitive values are not supposed to be stored through normal DM chat; use the local admin script instead

Current implications and limits:

- more shared continuity also means faster history growth, which can eventually cause retrieval quality drift or context rot if ranking stays naive
- bot-authored DM persistence increases storage and embedding cost over time
- relay memory currently exists in both `agent_actions` and raw `messages`, and tasks now share that same substrate, so future retrieval tuning has to manage duplication and priority carefully
- user-memory snapshots are bounded summaries, not source of truth, so stale summaries can create soft context rot if expiry and refresh stay naive
- explicit relay confirmation is safer than one-shot cross-user dispatch, but it adds friction and another expiring state machine to the DM UX
- this is still a permission-aware shared identity, not unrestricted global memory
- the DM tool planner is conservative by design, so user resolution can fail without an explicit mention or exact Discord name
- allowing guild/admin actions from DM raises the importance of conservative channel, role, and assignment resolution so Gigi does not mutate the wrong server resource from freeform text
- direct user grants are convenient, but they can drift away from your Discord role model if they are not reviewed periodically
- sensitive-data retrieval is safer than pushing secrets through OpenAI, but the current write path is intentionally outside Discord because sending new raw secret values through normal DM chat would otherwise leak them into ordinary history storage
- sensitive-data disclosure replies are intentionally not persisted into canonical message history, which protects values but also means later retrieval cannot rely on bot-authored history for those replies
- tool execution is still synchronous inside the DM turn, so this is not yet a durable worker or long-running orchestration system

## Development Scripts

```bash
npm run dev
npm run build
npm run typecheck
npm run register:commands
npm run supabase:start
npm run supabase:stop
npm run supabase:status
npm run supabase:db:reset
npm run supabase:db:push
npm run supabase:migration:new -- add_feature_name
npm run sensitive:data -- list --guild YOUR_GUILD_ID --owner YOUR_USER_ID
```

## Testing Guidance

Before opening a PR or deploying:

- Run `npm run typecheck`
- Run `npm run build`
- Run `terraform fmt -check` inside `terraform/` when infrastructure files change
- Run `terraform validate` inside `terraform/` after `terraform init -backend=false` when infrastructure files change
- Verify `/ping`
- Verify `/permission list`, `/permission grant`, and `/permission revoke`
- Verify `/ingestion status`, `/ingestion enable`, and `/ingestion disable` in a development Discord server
- Verify `/relay dm` creates a confirmation prompt, confirm it, and then verify that a participant can ask a follow-up in DM
- Verify `/task create`, `/task list`, and `/task complete`
- Verify `/assignment create` and `/assignment publish` in a development Discord server
- DM the bot with one direct question and one history-based question
- DM the bot with `what tools can you call?` and confirm the answer stays grounded to the actual runtime surface
- DM the bot with a relay request, then `confirm!`, and verify only one pending relay executes
- DM the bot with an ingestion request and an assignment request, and verify the same guild capability checks apply as they do through slash commands
- Seed a sensitive record with `npm run sensitive:data -- put --guild YOUR_GUILD_ID --owner YOUR_USER_ID --label github` and pipe the value through stdin, then DM the bot with `show my sensitive data` and `what is my github token`
- DM the bot with a raw sensitive-value write attempt like `remember my github token is ...` and confirm it refuses and skips normal history storage
- Confirm DM messages are stored in Supabase
- If you enable a row in `channel_ingestion_policies`, confirm guild messages for that channel are stored
- Check `GET /readyz` as well as `GET /healthz`
- Re-check all modified lines for secrets and private data leaks

## Human-Agent Workflow

For multi-terminal Codex work, use separate Git worktrees and explicit scope ownership.

- one Codex terminal should own one branch
- one branch should live in one worktree
- one worktree should have one clearly defined scope
- avoid parallel edits to the same files, shared service contracts, or Supabase migration
- for architecture-heavy work, analyze and define seams first, then parallelize implementation

The standing workflow is documented in [docs/human-agent-workflow.md](/Users/giancedrick/dev/projects/gigi/docs/human-agent-workflow.md).
When a human asks an agent what it is working on, the preferred answer format is also defined there.

## Project Structure

```text
src/
  commands/         Slash command handlers
  config/           Environment validation
  discord/          Client bootstrap and command registration
  lib/              Shared clients and logging
  services/         Control-plane, history, and DM retrieval services
  web/              Health server
deploy/
  nginx/            Reverse-proxy template for EC2
  systemd/          System service template for EC2
scripts/            EC2 bootstrap and deploy helpers
supabase/
  migrations/       SQL schema for the app
.codex/
  skills/           Repo-local agent workflow skills
docs/
  diagrams/         Durable architecture and flow diagrams
  discord-bot-plan.md
  agent-skills.md
  human-agent-workflow.md
  project-visualization-workflow.md
  setup.md
  credits.md
```

## Documentation

- [docs/discord-bot-plan.md](/Users/giancedrick/dev/projects/gigi/docs/discord-bot-plan.md)
- [docs/agent-skills.md](/Users/giancedrick/dev/projects/gigi/docs/agent-skills.md)
- [docs/human-agent-workflow.md](/Users/giancedrick/dev/projects/gigi/docs/human-agent-workflow.md)
- [docs/project-visualization-workflow.md](/Users/giancedrick/dev/projects/gigi/docs/project-visualization-workflow.md)
- [docs/architecture-v1.md](/Users/giancedrick/dev/projects/gigi/docs/architecture-v1.md)
- [docs/roadmap.md](/Users/giancedrick/dev/projects/gigi/docs/roadmap.md)
- [docs/setup.md](/Users/giancedrick/dev/projects/gigi/docs/setup.md)
- [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)
- [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md)
- [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md)
- [docs/credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md)
- [AGENTS.md](/Users/giancedrick/dev/projects/gigi/AGENTS.md)

## External Resources and Credits

This project requires attribution for external resources. Current credited resources are listed in [docs/credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md).
